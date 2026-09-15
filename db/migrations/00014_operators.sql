-- +goose Up
-- Minimal operator identity for attribution until Phase 17 introduces full
-- authentication and RBAC.
CREATE TABLE operators (
    id         text PRIMARY KEY,
    name       text NOT NULL,
    role       text NOT NULL DEFAULT 'operator' CHECK (role IN ('operator', 'supervisor', 'administrator', 'analyst')),
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE operators;
