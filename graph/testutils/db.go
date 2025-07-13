package testutils

import (
	"database/sql"

	"testing"

	_ "github.com/lib/pq"
)

var DB *sql.DB

// Returns already created DB instance
func SetupDB(t *testing.T) *sql.DB {
	t.Helper()
	if DB == nil {
		t.Fatal("DB is not initialized, do TestMain first.")
	}
	return DB
}
