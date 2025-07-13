// graph/main_test.go
package graph_test

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"

	"token_transfer/graph/tests/testutils"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var testDB *sql.DB

func TestMain(m *testing.M) {
	// Load .env file
	if err := godotenv.Load("../../.env"); err != nil {
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

	// Share testDB with other tests by testultis
	testutils.DB = testDB

	code := m.Run()

	_ = testDB.Close()
	os.Exit(code)
}
