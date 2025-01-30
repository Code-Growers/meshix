package main

//go:generate sqlc generate -f ../sqlc.yaml

import (
	"context"
	"database/sql"
	"fmt"
	meshixv1 "gen/proto/meshix/v1"
	"log"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"
	"server/internal/db"
	"server/internal/domain"
	"server/internal/handlers"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/pressly/goose/v3"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	_ "modernc.org/sqlite"
)

func main() {
	ctx := context.Background()
	if err := run(ctx); err != nil {
		slog.ErrorContext(ctx, "Main func exited with error", "err", err)
	}
}

var enabledLogging = true

func run(ctx context.Context) error {
	grpcPanicRecoveryHandler := func(p any) (err error) {
		slog.Error("gRPC recovered from panic", "panic", p, "stack", debug.Stack())
		return status.Errorf(codes.Internal, "%s", p)
	}

	minioClient, err := minio.New("localhost:9001", &minio.Options{
		Creds:  credentials.NewStaticV4("minio123", "minio123", ""),
		Secure: false,
	})
	if err != nil {
		return fmt.Errorf("Failed to create new minio client: %w", err)
	}

	_, err = minioClient.HealthCheck(10 * time.Second)
	if err != nil {
		return fmt.Errorf("Failed to start minio health check: %w", err)
	}

	dbPool, err := setupDB()
	if err != nil {
		return fmt.Errorf("Failed to setup db: %w", err)
	}

	meshix := Meshix{
		db: db.NewDatabase(dbPool),
	}

	interceptors := []grpc.UnaryServerInterceptor{}
	if enabledLogging {
		// TODO replace logger
		interceptors = append(interceptors, logging.UnaryServerInterceptor(InterceptorLogger(slog.Default()), logging.WithLogOnEvents(logging.StartCall, logging.FinishCall)))
	}

	interceptors = append(interceptors, recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(grpcPanicRecoveryHandler)))

	opts := []grpc.ServerOption{
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.ChainUnaryInterceptor(interceptors...),
	}
	grpcServer := grpc.NewServer(opts...)
	reflection.Register(grpcServer)
	meshixv1.RegisterMeshixServiceServer(grpcServer, &meshix)

	mux := mux.NewRouter()
	mux.HandleFunc("/cache/nix-cache-info", handlers.HandleNixCacheInfo)
	mux.Handle("/cache/nar/{hash}.nar.{compression}", handlers.HandlenNar(minioClient))
	mux.Handle("/cache/{hash}.narinfo", handlers.HandleNarInfo(minioClient))
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	slog.InfoContext(ctx, "Starting server", "addr", "0.0.0.0:8088")
	muxer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor == 2 && strings.Contains(r.Header.Get("Content-Type"), "application/grpc") {
			grpcServer.ServeHTTP(w, r)
		} else {
			mux.ServeHTTP(w, r)
		}
	})

	http2s := &http2.Server{}
	s := http.Server{
		Addr:              "0.0.0.0:8088",
		Handler:           h2c.NewHandler(muxer, http2s),
		ReadTimeout:       0,
		ReadHeaderTimeout: 0,
		WriteTimeout:      0,
		IdleTimeout:       0,
		MaxHeaderBytes:    0,
		ErrorLog:          log.Default(),
	}
	err = http2.ConfigureServer(&s, http2s)
	if err != nil {
		return fmt.Errorf("Failed to setup http2: %w", err)
	}

	err = s.ListenAndServe()
	if err != nil {
		return fmt.Errorf("gRPC Serve failed: %w", err)
	}

	return nil
}

// InterceptorLogger adapts slog logger to interceptor logger.
// This code is simple enough to be copied and not imported.
func InterceptorLogger(l *slog.Logger) logging.Logger {
	return logging.LoggerFunc(func(ctx context.Context, lvl logging.Level, msg string, fields ...any) {
		l.Log(ctx, slog.Level(lvl), msg, fields...)
	})
}

type Meshix struct {
	meshixv1.UnsafeMeshixServiceServer
	db db.Database
}

// ListPackages implements meshixv1.MeshixServiceServer.
func (m *Meshix) ListPackages(ctx context.Context, req *meshixv1.ListPackagesRequest) (*meshixv1.ListPackagesResponse, error) {
	packages, err := m.db.ListPackages(ctx)
	if err != nil {
		return nil, err
	}
	mappedPackages := []*meshixv1.Package{}
	for _, p := range packages {
		mappedPackages = append(mappedPackages, &meshixv1.Package{
			Name:    p.Name,
			Version: p.Version,
			NixMetadata: &meshixv1.NixMetadata{
				StorePath: p.NixMetadata.StorePath,
				MainBin:   p.NixMetadata.MainBin,
			},
		})
	}

	return &meshixv1.ListPackagesResponse{
		Packages: mappedPackages,
	}, nil
}

// PushPackage implements meshixv1.MeshixServiceServer.
func (m *Meshix) PushPackage(ctx context.Context, req *meshixv1.PushPackageRequest) (*meshixv1.PushPackageResponse, error) {
	err := m.db.PutPackage(ctx, domain.NewPackage{
		Name:    req.Package.Name,
		Version: req.Package.Version,
		NixMetadata: domain.NixMetadata{
			StorePath: req.Package.NixMetadata.StorePath,
			MainBin:   req.Package.NixMetadata.MainBin,
		},
	})
	if err != nil {
		return nil, err
	}

	return &meshixv1.PushPackageResponse{}, nil
}

var _ (meshixv1.MeshixServiceServer) = (*Meshix)(nil)

func setupDB() (*sql.DB, error) {
	conn, err := sql.Open("sqlite", "./data")
	if err != nil {
		return nil, fmt.Errorf("Failed to open sqlite db: %w", err)
	}
	conn.SetMaxOpenConns(1)

	g, err := goose.NewProvider("sqlite3", conn, os.DirFS("./migrations"))
	if err != nil {
		return nil, err
	}

	if _, err := g.Up(context.Background()); err != nil {
		return nil, fmt.Errorf("Failed to run goose db migrations: %w", err)
	}

	err = conn.Ping()
	if err != nil {
		return nil, err
	}

	return conn, nil
}
