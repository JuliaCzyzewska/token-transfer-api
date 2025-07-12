package graph

import (
	"context"
	"database/sql"
	"errors"
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

func assertBalances(t *testing.T, db *sql.DB, expectedA, expectedB int, addrA, addrB string) {
	t.Helper()

	a := getBalance(t, db, addrA)
	b := getBalance(t, db, addrB)

	t.Logf("Final balances: %s = %d, %s = %d", addrA, a, addrB, b)

	if a != expectedA || b != expectedB {
		t.Errorf("Unexpected balances: got %s = %d, %s = %d; want %s = %d, %s = %d",
			addrA, a, addrB, b, addrA, expectedA, addrB, expectedB)
	}
}

// Tests
func TestTransferBetweenExistingWallets(t *testing.T) {
	db := setupDB(t)

	ctx := context.Background()
	resolver := &Resolver{DB: db}
	mr := &mutationResolver{resolver}

	// Clean and seed test data
	clearWallets(t, db)
	initWallet(t, db, "A", 1000)
	initWallet(t, db, "B", 1000)

	// A -> B Transfer
	fromAddress := "A"
	toAddress := "B"
	amount := 100
	_, err := mr.Transfer(ctx, fromAddress, toAddress, int32(amount))
	if err != nil {
		t.Errorf("Transfer %s → %s failed: %v", fromAddress, toAddress, err)
	}

	// Check balances
	expectedA := 900
	expectedB := 1100
	assertBalances(t, db, expectedA, expectedB, "A", "B")

	// B -> A Transfer
	fromAddress = "B"
	toAddress = "A"
	amount = 100

	_, err = mr.Transfer(ctx, fromAddress, toAddress, int32(amount))
	if err != nil {
		t.Errorf("Transfer %s → %s failed: %v", fromAddress, toAddress, err)
	}

	// Check balances
	expectedA = 1000
	expectedB = 1000
	assertBalances(t, db, expectedA, expectedB, "A", "B")

}

func TestAddingNewWallet(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	resolver := &Resolver{DB: db}
	mr := &mutationResolver{resolver}

	// Clean data
	clearWallets(t, db)
	// Insert initial wallet
	fromAddress := "0x0000000000000000000000000000000000000000"
	initWallet(t, db, fromAddress, 1000000)

	// Add new wallet through transfer of tokens from initial wallet
	newWalletAddress := "A"
	amount := 100
	_, err := mr.Transfer(ctx, fromAddress, newWalletAddress, int32(amount))
	if err != nil {
		t.Errorf("Transfer %s → %s failed: %v", fromAddress, newWalletAddress, err)
	}

	// Check if new wallet exists
	newWalletBalance := getBalance(t, db, newWalletAddress)
	if newWalletBalance != amount {
		t.Errorf("Unexpected balance: got %d, want %d", newWalletBalance, amount)
	}
}

func TestTransferNoRowsError(t *testing.T) {
	db := setupDB(t)

	ctx := context.Background()
	resolver := &Resolver{DB: db}
	mr := &mutationResolver{resolver}

	// Clean and seed test data
	clearWallets(t, db)
	initWallet(t, db, "A", 1000)

	// Try transfering tokens from nonexistent sender
	fromAddress := "C"
	toAddress := "A"
	amount := 100
	_, err := mr.Transfer(ctx, fromAddress, toAddress, int32(amount))
	if err == nil {
		t.Fatal("Transfer from nonexistent sender did not throw error")
	}

	// Check if error is NoRows error
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Expected 'no rows' error, got: %v", err)
	}

}

func TestManyConcurrentTransfersDeadlock(t *testing.T) {
	db := setupDB(t)

	ctx := context.Background()
	resolver := &Resolver{DB: db}
	mr := &mutationResolver{resolver}

	// Clean and seed test data
	clearWallets(t, db)
	initWallet(t, db, "A", 1000)
	initWallet(t, db, "B", 1000)

	// wait for 50 wg.Done() before continuing
	const transferCount = 50
	var wg sync.WaitGroup
	wg.Add(transferCount)

	// Synchronization barrier
	// will wait until both goroutines are ready
	start := make(chan struct{})

	// Launch 50 transfers
	// 25 transfers A -> B (amount 5)
	// 25 transfers B -> A (amount 10)
	for i := 0; i < transferCount; i++ {
		// A -> B
		from := "A"
		to := "B"
		amount := 5

		//  B -> A
		if i%2 == 1 {
			from, to = "B", "A"
			amount = 10
		}

		// Transfer
		go func(from, to string, amount int) {
			defer wg.Done()
			<-start // barrier up

			_, err := mr.Transfer(ctx, from, to, int32(amount))
			if err != nil {
				t.Errorf("Transfer %s → %s failed: %v", from, to, err)
			}
		}(from, to, amount)
	}

	// Let all goroutines proceed at the same time
	close(start) // barier down

	// Wait for all to finish
	wg.Wait()

	// Check final balances
	// Expected:
	// A lost 25 × 5 = 125, gained 25 × 10 = 250; A = 1000 +125
	// B lost 25 × 10 = 250, gained 25 × 5 = 125; B = 1000 -125
	expectedA := 1125
	expectedB := 875

	assertBalances(t, db, expectedA, expectedB, "A", "B")

}
