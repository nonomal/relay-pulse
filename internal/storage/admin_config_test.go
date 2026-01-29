//go:build integration_postgres
// +build integration_postgres

// NOTE: These tests require a running PostgreSQL instance.
// They are skipped by default. To run them:
//   go test -tags=integration_postgres ./internal/storage/...
// with proper POSTGRES_* environment variables set.

package storage

import (
	"testing"
)

func TestAdminConfigTables(t *testing.T) {
	t.Skip("SQLite support removed, PostgreSQL integration tests not yet implemented")
}

func TestMonitorConfigCRUD(t *testing.T) {
	t.Skip("SQLite support removed, PostgreSQL integration tests not yet implemented")
}

func TestMonitorSecretCRUD(t *testing.T) {
	t.Skip("SQLite support removed, PostgreSQL integration tests not yet implemented")
}

func TestMonitorConfigAudit(t *testing.T) {
	t.Skip("SQLite support removed, PostgreSQL integration tests not yet implemented")
}

func TestBadgeDefinitionCRUD(t *testing.T) {
	t.Skip("SQLite support removed, PostgreSQL integration tests not yet implemented")
}

func TestBadgeBindingCRUD(t *testing.T) {
	t.Skip("SQLite support removed, PostgreSQL integration tests not yet implemented")
}
