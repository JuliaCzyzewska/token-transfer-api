package graph

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"sync"
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

func initWallet(t *testing.T, db *sql.DB, address string, balance int) {
	t.Helper()
	_, err := db.Exec("INSERT INTO wallets (address, token_balance) VALUES ($1, $2)", address, balance)
	if err != nil {
		t.Fatalf("Failed to insert wallet %s: %v", address, err)
	}
}

func clearWallets(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec("DELETE FROM wallets")
	if err != nil {
		t.Fatalf("Failed to clear wallets: %v", err)
	}
}

func getBalance(t *testing.T, db *sql.DB, address string) int {
	t.Helper()
	var balance int
	err := db.QueryRow("SELECT token_balance FROM wallets WHERE address = $1", address).Scan(&balance)
	if err != nil {
		t.Fatalf("Failed to get balance for %s: %v", address, err)
	}
	return balance
}

func TestConnection(t *testing.T) {
	db := setupDB(t)

	err := db.Ping()
	if err != nil {
		t.Fatalf("DB ping failed in test: %v", err)
	}

	t.Log("DB connection successful")

}

func TestConcurrentTransfersDeadlock(t *testing.T) {
	db := setupDB(t)

	ctx := context.Background()
	resolver := &Resolver{DB: db}
	mr := &mutationResolver{resolver}

	// Clean and seed test data
	clearWallets(t, db)
	initWallet(t, db, "A", 100)
	initWallet(t, db, "B", 100)

	// wait for 2 wg.Done() before continuing
	var wg sync.WaitGroup
	wg.Add(2)

	// Synchronization barrier
	// will wait until both goroutines are ready
	start := make(chan struct{})

	// Transfer A -> B
	go func() {
		defer wg.Done()
		<-start // barrier up
		_, err := mr.Transfer(ctx, "A", "B", 10)
		if err != nil {
			t.Errorf("Transfer A->B failed: %v", err)
		}
	}()

	// Transfer B -> A
	go func() {
		defer wg.Done()
		<-start // barrier up
		_, err := mr.Transfer(ctx, "B", "A", 20)
		if err != nil {
			t.Errorf("Transfer B->A failed: %v", err)
		}
	}()

	// Let both goroutines proceed at the same time
	close(start)

	// Wait for both to finish
	wg.Wait()

	// Check final balances
	a := getBalance(t, db, "A")
	b := getBalance(t, db, "B")

	t.Logf("Final balances: A = %d, B = %d", a, b)

	// Expected:
	// A lost 10, gained 20 = +10 => 110
	// B gained 10, lost 20 = -10 =>  90
	if a != 110 || b != 90 {
		t.Errorf("Unexpected final balances: A = %d, B = %d", a, b)
	}
}
