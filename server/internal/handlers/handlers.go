package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"server/internal/config"

	"github.com/gorilla/mux"
	"github.com/minio/minio-go/v7"
	"github.com/nix-community/go-nix/pkg/narinfo"
)

var cacheInfo = `WantMassQuery: 1
StoreDir: /nix/store
Priority: 39
`

// Nix cache information.
//
// An example of a correct response is as follows:
//
// ```text
// StoreDir: /nix/store
// WantMassQuery: 1
// Priority: 40
// ```
func HandleNixCacheInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "text/x-nix-cache-info")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte(cacheInfo))
	if err != nil {
		slog.ErrorContext(r.Context(), "Failed to write cache info", "err", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func HandleNarInfo(client *minio.Client, cacheCfg config.BinaryCacheCfg, s3Cfg config.MinioCfg) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		fmt.Printf("NAR Info Method: %+v Path: %+v\n", r.Method, r.URL.Path)
		vars := mux.Vars(r)
		hash := vars["hash"]
		if r.Method == http.MethodHead {
			slog.InfoContext(ctx, "Heading narinfo", "hash", hash)
			obj, err := client.GetObject(ctx, s3Cfg.Bucket, hash+".narinfo", minio.GetObjectOptions{})
			if err != nil {
				slog.ErrorContext(ctx, "Failed to head narinfo", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			defer obj.Close()
			_, err = obj.Stat()
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
		}

		if r.Method == http.MethodGet {
			slog.InfoContext(ctx, "Getting narinfo", "hash", hash)
			obj, err := client.GetObject(context.Background(), s3Cfg.Bucket, hash+".narinfo", minio.GetObjectOptions{})
			if err != nil {
				slog.ErrorContext(ctx, "Failed to get nar info", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, err = obj.Stat()
			if err != nil {
				slog.ErrorContext(ctx, "Failed to stat narinfo", "err", err)
				w.WriteHeader(http.StatusNotFound)
				return
			}

			defer obj.Close()
			_, err = io.Copy(w, obj)
			if err != nil {
				merr := minio.ToErrorResponse(err)
				slog.ErrorContext(ctx, "Failed to copy narinfo to response", "err", err)
				if merr.StatusCode != 0 {
					w.WriteHeader(merr.StatusCode)
				} else {
					w.WriteHeader(http.StatusInternalServerError)
				}
				return
			}
			err = obj.Close()
			if err != nil {
				slog.ErrorContext(ctx, "Failed to close narinfo s3", "err", err)
			}
			return
		}
		if r.Method == http.MethodPut {
			slog.InfoContext(ctx, "Uploading narinfo", "hash", hash, "length", r.ContentLength)
			info, err := narinfo.Parse(r.Body)
			if err != nil {
				slog.ErrorContext(ctx, "Failed to parse narinfo", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			r.Body.Close()

			sig, err := cacheCfg.PrivateKey.Sign(nil, info.Fingerprint())
			if err != nil {
				slog.ErrorContext(ctx, "Failed to sign narinfo", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			info.Signatures = append(info.Signatures, sig)

			narinfoFile := bytes.NewBuffer([]byte(info.String()))
			uploadInfo, err := client.PutObject(ctx, s3Cfg.Bucket, hash+".narinfo", narinfoFile, int64(narinfoFile.Len()), minio.PutObjectOptions{})
			if err != nil {
				slog.ErrorContext(ctx, "Failed to upload nar info", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			slog.InfoContext(ctx, "Successful upload", "info", uploadInfo.Key)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
}

func HandlenNar(client *minio.Client, minioCfg config.MinioCfg) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		fmt.Printf("NAR Method: %+v Path: %+v\n", r.Method, r.URL.Path)
		vars := mux.Vars(r)
		hash := vars["hash"]
		compression := vars["compression"]
		if !isCompressionSupported(compression) {
			panic(fmt.Sprintf("Unknown compression: %v", compression))
		}
		if r.Method == http.MethodHead {
			slog.InfoContext(ctx, "Heading nar", "hash", hash)
			// TODO move bucket to cfg
			obj, err := client.GetObject(ctx, minioCfg.Bucket, hash+".nar", minio.GetObjectOptions{})
			if err != nil {
				slog.ErrorContext(r.Context(), "Failed to get nar", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			defer obj.Close()
			_, err = obj.Stat()
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			return
		}
		if r.Method == http.MethodGet {
			slog.InfoContext(ctx, "Getting nar", "hash", hash)
			obj, err := client.GetObject(ctx, minioCfg.Bucket, hash+".nar", minio.GetObjectOptions{})
			if err != nil {
				slog.ErrorContext(r.Context(), "Failed to get nar", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			_, err = obj.Stat()
			if err != nil {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			w.Header().Add("content-type", "application/x-nix-nar")
			compressedW, err := NewCompressionWriter(compression, w)
			if err != nil {
				slog.ErrorContext(ctx, "Failed to create compression writer", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			_, err = io.Copy(compressedW, obj)
			if err != nil {
				merr := minio.ToErrorResponse(err)
				slog.ErrorContext(r.Context(), "Failed to copy nar to response", "err", err)
				w.WriteHeader(merr.StatusCode)
				return
			}

			err = compressedW.Close()
			if err != nil {
				slog.ErrorContext(r.Context(), "Failed to close compressed writter", "err", err)
			}
			err = obj.Close()
			if err != nil {
				slog.ErrorContext(r.Context(), "Failed to close nar s3", "err", err)
			}
			return
		}
		if r.Method == http.MethodPut {
			slog.InfoContext(ctx, "Uploading nar", "hash", hash, "length", r.ContentLength)
			body := bytes.NewBuffer([]byte{})
			compressionR, err := NewCompressionReader(compression, r.Body)
			if err != nil {
				slog.ErrorContext(ctx, "Failed to create compression reader", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			_, err = io.Copy(body, compressionR)
			if err != nil {
				slog.ErrorContext(ctx, "Failed to read whole body of nar", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			err = r.Body.Close()
			if err != nil {
				slog.ErrorContext(ctx, "Failed to read whole body of nar", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			uploadInfo, err := client.PutObject(context.Background(), minioCfg.Bucket, hash+".nar", body, int64(body.Len()), minio.PutObjectOptions{})
			if err != nil {
				slog.ErrorContext(r.Context(), "Failed to upload nar", "err", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			slog.InfoContext(ctx, "Successful upload", "info", uploadInfo.Key)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
}
