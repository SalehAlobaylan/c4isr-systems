-- +goose Up
ALTER TABLE platform_worker_status
 ADD COLUMN scratch_bytes bigint CHECK (scratch_bytes BETWEEN 0 AND 1099511627776),
 ADD COLUMN toolchain_sha256 text CHECK (toolchain_sha256 ~ '^[0-9a-f]{64}$');

-- +goose Down
ALTER TABLE platform_worker_status DROP COLUMN toolchain_sha256, DROP COLUMN scratch_bytes;
