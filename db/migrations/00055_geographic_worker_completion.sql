-- +goose Up
ALTER TABLE geographic_environment_releases
 ADD COLUMN publication_lease_sha256 text CHECK(publication_lease_sha256 ~ '^[0-9a-f]{64}$');

-- +goose Down
ALTER TABLE geographic_environment_releases DROP COLUMN publication_lease_sha256;
