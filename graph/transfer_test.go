package graph

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	// Load .env file
	if err := godotenv.Load("../.env"); err != nil {
		log.Fatalf("Failed to load .env file: %v", err)
	}

	// Build DB connection string
	connStr := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%s sslmode=disable",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
	)

	// Open DB connection
	var err error
	testDB, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}

	// Check if DB is reachable
	if err := testDB.Ping(); err != nil {
		log.Fatalf("Failed to ping DB: %v", err)
	}

	// Reset table in DB
	if err := recreateTable(testDB); err != nil {
		log.Fatalf("Failed to recreate table: %v", err)
	}

	// Run the tests
	code := m.Run()

	// Close DB
	if err := testDB.Close(); err != nil {
		log.Printf("Failed to close DB: %v", err)
	}

	os.Exit(code)

}

func recreateTable(db *sql.DB) error {
	// Drop table if exists
	_, err := db.Exec(`DROP TABLE IF EXISTS wallets`)
	if err != nil {
		return fmt.Errorf("error dropping table: %w", err)
	}

	// Create table
	_, err = db.Exec(`
		CREATE TABLE wallets (
			address TEXT PRIMARY KEY,
			token_balance INTEGER NOT NULL CHECK (token_balance >= 0)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating table: %w", err)
	}

	// Insert initial wallet
	_, err = db.Exec(`
		INSERT INTO wallets (address, token_balance)
		VALUES ('0x0000000000000000000000000000000000000000', 1000000)
	`)
	if err != nil {
		return fmt.Errorf("error inserting initial wallet: %w", err)
	}

	return nil
}

// Returns already created DB instance
func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	return testDB
}

func TestConnection(t *testing.T) {
	db := setupDB(t)

	err := db.Ping()
	if err != nil {
		t.Fatalf("DB ping failed in test: %v", err)
	}

	t.Log("DB connection successful")

}
