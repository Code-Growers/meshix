package db

import (
	"context"
	"database/sql"
	sqlite_queries "server/internal/db/sqlite_generated"
	"server/internal/domain"
)

type Database interface {
	PutPackage(ctx context.Context, pkg domain.NewPackage) error
	ListPackages(ctx context.Context) ([]domain.Package, error)
}

func NewDatabase(pool *sql.DB) Database {
	return &sqliteDatabase{
		q: sqlite_queries.New(pool),
	}
}

type sqliteDatabase struct {
	q *sqlite_queries.Queries
}

// ListPackages implements Database.
func (s *sqliteDatabase) ListPackages(ctx context.Context) ([]domain.Package, error) {
	packages, err := s.q.ListPackages(ctx)
	if err != nil {
		return nil, err
	}
	mappedPackages := []domain.Package{}
	for _, p := range packages {
		mappedPackages = append(mappedPackages, domain.Package{
			Name:    p.Package.Name,
			Version: p.Package.Version,
			NixMetadata: domain.NixMetadata{
				StorePath: p.Package.NixStoreHash,
				MainBin:   p.Package.NixMainBin,
			},
		})
	}

	return mappedPackages, nil
}

// PutPackage implements Database.
func (s *sqliteDatabase) PutPackage(ctx context.Context, pkg domain.NewPackage) error {
	err := s.q.InsertPackage(ctx, sqlite_queries.InsertPackageParams{
		Name:         pkg.Name,
		Version:      pkg.Version,
		NixStoreHash: pkg.NixMetadata.StorePath,
		NixMainBin:   pkg.NixMetadata.MainBin,
	})
	if err != nil {
		return err
	}

	return nil
}

var _ (Database) = (*sqliteDatabase)(nil)
