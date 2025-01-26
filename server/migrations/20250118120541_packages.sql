-- +goose Up
-- +goose StatementBegin
CREATE TABLE packages (
    id integer PRIMARY KEY,

    name TEXT NOT NULL,
    version TEXT NOT NULL,
    nix_store_hash TEXT NOT NULL,
    nix_main_bin TEXT NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE packages;
-- +goose StatementEnd
