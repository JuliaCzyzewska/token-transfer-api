package testutils

import (
	"database/sql"
	"testing"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func ResetDatabaseState(db *sql.DB) error {
	_, err := db.Exec("DELETE FROM wallets")
	if err != nil {
		return err
	}
	_, err = db.Exec(`
		INSERT INTO wallets (address, token_balance)
		VALUES ($1, $2::numeric)
	`, "0x0000000000000000000000000000000000000000", "1000000")
	return err
}

// Returns already created DB instance
func SetupDB(t *testing.T) *sql.DB {
	t.Helper()
	if DB == nil {
		t.Fatal("DB is not initialized, do TestMain first.")
	}
	return DB
}
