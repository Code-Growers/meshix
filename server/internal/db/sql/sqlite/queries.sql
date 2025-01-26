-- name: InsertPackage :exec
INSERT INTO packages (
    name,
    version,
    nix_store_hash,
    nix_main_bin
) VALUES(
 sqlc.arg(name),
 sqlc.arg(version),
 sqlc.arg(nix_store_hash),
 sqlc.arg(nix_main_bin)
);

-- name: ListPackages :many
SELECT sqlc.embed(packages)
 FROM packages;
