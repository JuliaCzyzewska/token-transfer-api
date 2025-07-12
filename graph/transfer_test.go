package graph

import (
	"context"
	"database/sql"

	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"sync"
	"testing"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/shopspring/decimal"
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

	// Run the tests
	code := m.Run()

	// Close DB
	if err := testDB.Close(); err != nil {
		log.Printf("Failed to close DB: %v", err)
	}

	os.Exit(code)

}

// Returns already created DB instance
func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	return testDB
}

func initWallet(t *testing.T, db *sql.DB, address string, balance string) {
	t.Helper()
	_, err := db.Exec("INSERT INTO wallets (address, token_balance) VALUES ($1, $2::numeric)", address, balance)
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

func getBalance(t *testing.T, db *sql.DB, address string) string {
	t.Helper()
	var balance string
	err := db.QueryRow("SELECT token_balance FROM wallets WHERE address = $1", address).Scan(&balance)
	if err != nil {
		t.Fatalf("Failed to get balance for %s: %v", address, err)
	}
	return balance
}

func assertBalance(t *testing.T, db *sql.DB, expectedA, addrA string) {
	t.Helper()
	aStr := getBalance(t, db, addrA)

	// Convert balance strings into decimals
	aDec, err := decimal.NewFromString(aStr)
	if err != nil {
		t.Fatalf("Invalid decimal in DB balance for %s: %v", addrA, err)
	}

	expectedADec, err := decimal.NewFromString(expectedA)
	if err != nil {
		t.Fatalf("Invalid decimal in expected balance for %s: %v", addrA, err)
	}

	// Check balance
	t.Logf("Final balance: %s = %s", addrA, aDec.String())

	if !aDec.Equal(expectedADec) {
		t.Errorf("Unexpected balance: got %s = %s; want %s = %s",
			addrA, aDec.String(), addrA, expectedADec.String())
	}

}

func doTransfer(t *testing.T, mr *mutationResolver, ctx context.Context, fromAddress, toAddress, amount string) {
	t.Helper()

	_, err := mr.Transfer(ctx, fromAddress, toAddress, amount)
	if err != nil {
		t.Errorf("Transfer %s → %s failed: %v", fromAddress, toAddress, err)
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
	initWallet(t, db, "A", "1000")
	initWallet(t, db, "B", "1000")

	// A -> B Transfer
	fromAddress := "A"
	toAddress := "B"
	amount := "100"
	doTransfer(t, mr, ctx, fromAddress, toAddress, amount)

	// Check balances
	expectedA := "900"
	expectedB := "1100"
	assertBalance(t, db, expectedA, "A")
	assertBalance(t, db, expectedB, "B")

	// B -> A Transfer
	fromAddress = "B"
	toAddress = "A"
	amount = "100"
	doTransfer(t, mr, ctx, fromAddress, toAddress, amount)

	// Check balances
	expectedA = "1000"
	expectedB = "1000"
	assertBalance(t, db, expectedA, "A")
	assertBalance(t, db, expectedB, "B")

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
	initWallet(t, db, fromAddress, "1000000")

	// Add new wallet through transfer of tokens from initial wallet
	newWalletAddress := "A"
	amount := "100"
	doTransfer(t, mr, ctx, fromAddress, newWalletAddress, amount)

	// Check if new wallet exists
	assertBalance(t, db, amount, newWalletAddress)

}

func TestFractionalTokenTransfer(t *testing.T) {
	db := setupDB(t)
	ctx := context.Background()
	resolver := &Resolver{DB: db}
	mr := &mutationResolver{resolver}

	// Clean data
	clearWallets(t, db)
	// Insert initial wallet
	fromAddress := "0x0000000000000000000000000000000000000000"
	initWallet(t, db, fromAddress, "1000000")

	toAddress := "A"
	amount := "0.000000000000000001" // 1 * 10^-18
	doTransfer(t, mr, ctx, fromAddress, toAddress, amount)

	// Check balances
	expectedSenderBalance := "999999.999999999999999999"
	assertBalance(t, db, amount, toAddress)
	assertBalance(t, db, expectedSenderBalance, fromAddress)
}

func TestTransferNoRowsError(t *testing.T) {
	db := setupDB(t)

	ctx := context.Background()
	resolver := &Resolver{DB: db}
	mr := &mutationResolver{resolver}

	// Clean and seed test data
	clearWallets(t, db)
	initWallet(t, db, "A", "1000")

	// Try transfering tokens from nonexistent sender
	fromAddress := "C"
	toAddress := "A"
	amount := "100"
	_, err := mr.Transfer(ctx, fromAddress, toAddress, amount)
	// Check if transfer throws error
	if err == nil {
		t.Fatal("Transfer from nonexistent sender did not throw error")
	}

	// Check error type
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Expected 'no rows' error, got: %v", err)
	}
}

func TestTransferReducesBalanceToZero(t *testing.T) {
	db := setupDB(t)

	ctx := context.Background()
	resolver := &Resolver{DB: db}
	mr := &mutationResolver{resolver}

	// Clean and seed test data
	clearWallets(t, db)
	amount := "1000"
	initWallet(t, db, "A", amount)

	// Transfer
	fromAddress := "A"
	toAddress := "B"
	doTransfer(t, mr, ctx, fromAddress, toAddress, amount)

	// Check balances
	expectedA := "0"
	expectedB := "1000"
	assertBalance(t, db, expectedA, "A")
	assertBalance(t, db, expectedB, "B")

}

func TestTransferInsufficientBalanceError(t *testing.T) {
	db := setupDB(t)

	ctx := context.Background()
	resolver := &Resolver{DB: db}
	mr := &mutationResolver{resolver}

	// Clean and seed test data
	clearWallets(t, db)
	initWallet(t, db, "A", "1000")

	// Transfer
	fromAddress := "A"
	toAddress := "B"
	_, err := mr.Transfer(ctx, fromAddress, toAddress, "1100")
	// Check if transfer throws error
	if err == nil {
		t.Fatal("Transfer with insufficient balance did not throw error")
	}

	// Check error type
	if !strings.Contains(err.Error(), "insufficient balance") {
		t.Fatalf("Expected 'insufficient balance' error, got: %v", err)
	}
}

func TestTransferAfterInsufficientBalance(t *testing.T) {
	db := setupDB(t)

	ctx := context.Background()
	resolver := &Resolver{DB: db}
	mr := &mutationResolver{resolver}

	// Clean and seed test data
	clearWallets(t, db)
	initWallet(t, db, "A", "10")

	// Transfer amount bigger than sender's balance
	fromAddress := "A"
	toAddress := "B"
	amount := "11"

	_, err := mr.Transfer(ctx, fromAddress, toAddress, amount)
	// Check if transfer throws error
	if err == nil {
		t.Fatal("Transfer with insufficient balance did not throw error")
	}
	// Check error type
	if !strings.Contains(err.Error(), "insufficient balance") {
		t.Fatalf("Expected 'insufficient balance' error, got: %v", err)
	}

	// Transfer amount sender can send
	amount = "10"
	doTransfer(t, mr, ctx, fromAddress, toAddress, amount)

	// Check balances
	expectedA := "0"
	expectedB := "10"
	assertBalance(t, db, expectedA, "A")
	assertBalance(t, db, expectedB, "B")

}

func TestCyclicTransfer(t *testing.T) {
	db := setupDB(t)

	ctx := context.Background()
	resolver := &Resolver{DB: db}
	mr := &mutationResolver{resolver}

	// Clean and seed test data
	clearWallets(t, db)
	initWallet(t, db, "A", "10")

	// A -> B Transfer
	amount := "10"
	fromAddress := "A"
	toAddress := "B"
	doTransfer(t, mr, ctx, fromAddress, toAddress, amount)

	// B -> C Transfer
	fromAddress = "B"
	toAddress = "C"
	doTransfer(t, mr, ctx, fromAddress, toAddress, amount)

	// C -> A Transfer
	fromAddress = "C"
	toAddress = "A"
	doTransfer(t, mr, ctx, fromAddress, toAddress, amount)

	// Check balances
	expectedA := "10"
	expectedB := "0"
	expectedC := "0"

	assertBalance(t, db, expectedA, "A")
	assertBalance(t, db, expectedB, "B")
	assertBalance(t, db, expectedC, "C")

}

func TestRaceConditionSameWalletConcurrentTransfers(t *testing.T) {
	db := setupDB(t)

	ctx := context.Background()
	resolver := &Resolver{DB: db}
	mr := &mutationResolver{resolver}

	// Clean and seed test data
	clearWallets(t, db)
	initWallet(t, db, "A", "10")
	initWallet(t, db, "D", "10")

	// wait for 3 wg.Done() before continuing
	var wg sync.WaitGroup
	wg.Add(3)

	// Synchronization barrier
	// wait until all goroutines are ready
	start := make(chan struct{})

	// Transfer: A -> B (amount 4)
	// can fail due to insufficent balance
	go func() {
		defer wg.Done()
		<-start // barrier up
		_, err := mr.Transfer(ctx, "A", "B", "4")
		if err != nil && !strings.Contains(err.Error(), "insufficient balance") {
			t.Errorf("A -> B failed unexpectedly: %v", err)
		}
	}()

	// Transfer: A -> C (amount 7)
	// can fail due to insufficent balance
	go func() {
		defer wg.Done()
		<-start // barrier up
		_, err := mr.Transfer(ctx, "A", "C", "7")
		if err != nil && !strings.Contains(err.Error(), "insufficient balance") {
			t.Errorf("A -> C failed unexpectedly: %v", err)
		}
	}()

	// Transfer: D -> A (amount 1)
	go func() {
		defer wg.Done()
		<-start // barrier up
		_, err := mr.Transfer(ctx, "D", "A", "1")
		if err != nil {
			t.Errorf("D -> A failed unexpectedly: %v", err)
		}
	}()

	close(start) // bariers down

	// Wait for all to finish
	wg.Wait()

	// Check final balances
	aBalance := getBalance(t, db, "A")
	bBalance := getBalance(t, db, "B")
	cBalance := getBalance(t, db, "C")

	t.Logf("Final balances: A = %s, B = %s, C = %s", aBalance, bBalance, cBalance)

	// Convert balance string into decimal
	aDec, err := decimal.NewFromString(aBalance)
	if err != nil {
		t.Fatalf("Invalid decimal in DB balance for %s: %v", "A", err)
	}

	// Expected:
	// Final A wallet balance should be between [0, 11]
	lowerBound := decimal.NewFromInt(0)
	upperBound := decimal.NewFromInt(10)

	if aDec.LessThan(lowerBound) || aDec.GreaterThan(upperBound) {
		t.Errorf("Balance A should never go below 0 or above 11, got %s", aBalance)
	}
}

func TestManyConcurrentTransfersDeadlock(t *testing.T) {
	db := setupDB(t)

	ctx := context.Background()
	resolver := &Resolver{DB: db}
	mr := &mutationResolver{resolver}

	// Clean and seed test data
	clearWallets(t, db)
	initWallet(t, db, "A", "1000")
	initWallet(t, db, "B", "1000")

	// wait for 50 wg.Done() before continuing
	const transferCount = 50
	var wg sync.WaitGroup
	wg.Add(transferCount)

	// Synchronization barrier
	// wait until both goroutines are ready
	start := make(chan struct{})

	// Launch 50 transfers
	// 25 transfers A -> B (amount 5)
	// 25 transfers B -> A (amount 10)
	for i := 0; i < transferCount; i++ {
		// A -> B
		fromAddress := "A"
		toAddress := "B"
		amount := "5"

		//  B -> A
		if i%2 == 1 {
			fromAddress, toAddress = "B", "A"
			amount = "10"
		}

		// Transfer
		go func(from, to string, amount string) {
			defer wg.Done()
			<-start // barrier up

			doTransfer(t, mr, ctx, fromAddress, toAddress, amount)
		}(fromAddress, toAddress, amount)
	}

	// Let all goroutines proceed at the same time
	close(start) // bariers down

	// Wait for all to finish
	wg.Wait()

	// Check final balances
	// Expected:
	// A lost 25 × 5 = 125, gained 25 × 10 = 250; A = 1000 +125
	// B lost 25 × 10 = 250, gained 25 × 5 = 125; B = 1000 -125
	expectedA := "1125"
	expectedB := "875"

	assertBalance(t, db, expectedA, "A")
	assertBalance(t, db, expectedB, "B")
}
