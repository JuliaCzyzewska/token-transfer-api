package graph_test

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"

	"token_transfer/graph"
	"token_transfer/graph/tests/testutils"

	_ "github.com/lib/pq"
	"github.com/shopspring/decimal"
)

// Tests
func TestTransferBetweenExistingWallets(t *testing.T) {
	db := testutils.SetupDB(t)

	ctx := context.Background()
	resolver := &graph.Resolver{
		DB:          db,
		WalletTable: "test_wallets",
	}

	mutation := resolver.Mutation()

	aAddress := "0xA000000000000000000000000000000000000000"
	bAddress := "0xB000000000000000000000000000000000000000"

	// Clean and seed test data
	clearWallets(t, db)

	initWallet(t, db, aAddress, "1000")
	initWallet(t, db, bAddress, "1000")

	// A -> B Transfer
	fromAddress := aAddress
	toAddress := bAddress
	amount := "100"
	doTransfer(t, mutation, ctx, fromAddress, toAddress, amount)

	// Check balances
	expectedA := "900"
	expectedB := "1100"
	assertBalance(t, db, expectedA, aAddress)
	assertBalance(t, db, expectedB, bAddress)

	// B -> A Transfer
	fromAddress = bAddress
	toAddress = aAddress
	amount = "100"
	doTransfer(t, mutation, ctx, fromAddress, toAddress, amount)

	// Check balances
	expectedA = "1000"
	expectedB = "1000"
	assertBalance(t, db, expectedA, aAddress)
	assertBalance(t, db, expectedB, bAddress)

}

func TestAddingNewWallet(t *testing.T) {
	db := testutils.SetupDB(t)
	ctx := context.Background()
	resolver := &graph.Resolver{
		DB:          db,
		WalletTable: "test_wallets",
	}

	mutation := resolver.Mutation()

	aAddress := "0xA000000000000000000000000000000000000000"

	// Clean data
	clearWallets(t, db)
	// Insert initial wallet
	fromAddress := "0x0000000000000000000000000000000000000000"
	initWallet(t, db, fromAddress, "1000000")

	// Add new wallet through transfer of tokens from initial wallet
	newWalletAddress := aAddress
	amount := "100"
	doTransfer(t, mutation, ctx, fromAddress, newWalletAddress, amount)

	// Check if new wallet exists
	assertBalance(t, db, amount, newWalletAddress)

}

func TestFractionalTokenTransfer(t *testing.T) {
	db := testutils.SetupDB(t)
	ctx := context.Background()
	resolver := &graph.Resolver{
		DB:          db,
		WalletTable: "test_wallets",
	}

	mutation := resolver.Mutation()

	// Clean data
	clearWallets(t, db)
	// Insert initial wallet
	fromAddress := "0x0000000000000000000000000000000000000000"
	initWallet(t, db, fromAddress, "1000000")

	aAddress := "0xA000000000000000000000000000000000000000"
	toAddress := aAddress
	amount := "0.000000000000000001" // 1 * 10^-18
	doTransfer(t, mutation, ctx, fromAddress, toAddress, amount)

	// Check balances
	expectedSenderBalance := "999999.999999999999999999"
	assertBalance(t, db, amount, toAddress)
	assertBalance(t, db, expectedSenderBalance, fromAddress)
}

func TestTransferNoRowsError(t *testing.T) {
	db := testutils.SetupDB(t)

	ctx := context.Background()
	resolver := &graph.Resolver{
		DB:          db,
		WalletTable: "test_wallets",
	}

	mutation := resolver.Mutation()

	aAddress := "0xA000000000000000000000000000000000000000"
	cAddress := "0xC000000000000000000000000000000000000000"

	// Clean and seed test data
	clearWallets(t, db)
	initWallet(t, db, aAddress, "1000")

	// Try transfering tokens from nonexistent sender
	fromAddress := cAddress
	toAddress := aAddress
	amount := "100"
	_, err := mutation.Transfer(ctx, fromAddress, toAddress, amount)
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
	db := testutils.SetupDB(t)

	ctx := context.Background()
	resolver := &graph.Resolver{
		DB:          db,
		WalletTable: "test_wallets",
	}

	mutation := resolver.Mutation()

	aAddress := "0xA000000000000000000000000000000000000000"
	bAddress := "0xB000000000000000000000000000000000000000"

	// Clean and seed test data
	clearWallets(t, db)
	amount := "1000"
	initWallet(t, db, aAddress, amount)

	// Transfer
	fromAddress := aAddress
	toAddress := bAddress
	doTransfer(t, mutation, ctx, fromAddress, toAddress, amount)

	// Check balances
	expectedA := "0"
	expectedB := "1000"
	assertBalance(t, db, expectedA, aAddress)
	assertBalance(t, db, expectedB, bAddress)

}

func TestTransferInsufficientBalanceError(t *testing.T) {
	db := testutils.SetupDB(t)

	ctx := context.Background()
	resolver := &graph.Resolver{
		DB:          db,
		WalletTable: "test_wallets",
	}

	mutation := resolver.Mutation()

	aAddress := "0xA000000000000000000000000000000000000000"
	bAddress := "0xB000000000000000000000000000000000000000"

	// Clean and seed test data
	clearWallets(t, db)
	initWallet(t, db, aAddress, "1000")

	// Transfer
	fromAddress := aAddress
	toAddress := bAddress
	_, err := mutation.Transfer(ctx, fromAddress, toAddress, "1100")
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
	db := testutils.SetupDB(t)

	ctx := context.Background()
	resolver := &graph.Resolver{
		DB:          db,
		WalletTable: "test_wallets",
	}

	mutation := resolver.Mutation()

	aAddress := "0xA000000000000000000000000000000000000000"
	bAddress := "0xB000000000000000000000000000000000000000"

	// Clean and seed test data
	clearWallets(t, db)
	initWallet(t, db, aAddress, "10")

	// Transfer amount bigger than sender's balance
	fromAddress := aAddress
	toAddress := bAddress
	amount := "11"

	_, err := mutation.Transfer(ctx, fromAddress, toAddress, amount)
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
	doTransfer(t, mutation, ctx, fromAddress, toAddress, amount)

	// Check balances
	expectedA := "0"
	expectedB := "10"
	assertBalance(t, db, expectedA, aAddress)
	assertBalance(t, db, expectedB, bAddress)

}

func TestValidateTokenAmount_InvalidDecimal(t *testing.T) {
	db := testutils.SetupDB(t)

	ctx := context.Background()
	resolver := &graph.Resolver{
		DB:          db,
		WalletTable: "test_wallets",
	}

	mutation := resolver.Mutation()

	aAddress := "0xA000000000000000000000000000000000000000"
	bAddress := "0xB000000000000000000000000000000000000000"

	// Clean and seed test data
	clearWallets(t, db)
	initWallet(t, db, aAddress, "10")

	// Transfer
	invalidAmount := "abc123"
	_, err := mutation.Transfer(ctx, aAddress, bAddress, invalidAmount)

	// Check if transfer throws error
	if err == nil {
		t.Fatal("Transfer with invalid amount did not throw error")
	}
	// Check error type
	if !strings.Contains(err.Error(), "invalid decimal amount") {
		t.Fatalf("Expected 'invalid decimal amount' error, got: %v", err)
	}
}

func TestValidateAmount_TooManyDecimalPlaces(t *testing.T) {
	db := testutils.SetupDB(t)

	ctx := context.Background()
	resolver := &graph.Resolver{
		DB:          db,
		WalletTable: "test_wallets",
	}

	mutation := resolver.Mutation()

	aAddress := "0xA000000000000000000000000000000000000000"
	bAddress := "0xB000000000000000000000000000000000000000"

	// Clean and seed test data
	clearWallets(t, db)
	initWallet(t, db, "A", "10")

	// Transfer
	invalidAmount := "1.1234567890123456789" // >18 decimal places
	_, err := mutation.Transfer(ctx, aAddress, bAddress, invalidAmount)

	// Check if transfer throws error
	if err == nil {
		t.Fatal("Transfer with invalid amount did not throw error")
	}
	// Check error type
	if !strings.Contains(err.Error(), "too many decimal places") {
		t.Fatalf("Expected 'too many decimal places' error, got: %v", err)
	}

}

func TestValidateAmount_TooManyDigits(t *testing.T) {
	db := testutils.SetupDB(t)

	ctx := context.Background()
	resolver := &graph.Resolver{
		DB:          db,
		WalletTable: "test_wallets",
	}

	mutation := resolver.Mutation()

	aAddress := "0xA000000000000000000000000000000000000000"
	bAddress := "0xB000000000000000000000000000000000000000"

	// Clean and seed test data
	clearWallets(t, db)
	initWallet(t, db, aAddress, "10")

	// Transfer
	invalidAmount := "12345678901234567890123456789.0" // >28 digits
	_, err := mutation.Transfer(ctx, aAddress, bAddress, invalidAmount)

	// Check if transfer throws error
	if err == nil {
		t.Fatal("Transfer with invalid amount did not throw error")
	}
	// Check error type
	if !strings.Contains(err.Error(), "too many digits") {
		t.Fatalf("Expected 'too many digits' error, got: %v", err)
	}

}

func TestValidateAmount_AmountBelowZero(t *testing.T) {
	db := testutils.SetupDB(t)

	ctx := context.Background()
	resolver := &graph.Resolver{
		DB:          db,
		WalletTable: "test_wallets",
	}

	mutation := resolver.Mutation()

	aAddress := "0xA000000000000000000000000000000000000000"
	bAddress := "0xB000000000000000000000000000000000000000"

	// Clean and seed test data
	clearWallets(t, db)
	initWallet(t, db, aAddress, "10")

	// Transfer
	invalidAmount := "-12"
	_, err := mutation.Transfer(ctx, aAddress, bAddress, invalidAmount)

	// Check if transfer throws error
	if err == nil {
		t.Fatal("Transfer with invalid amount did not throw error")
	}
	// Check error type
	if !strings.Contains(err.Error(), "amount must be greater than zero") {
		t.Fatalf("Expected 'amount must be greater than zero' error, got: %v", err)
	}

}

func TestValidateAddressess_SameAddress(t *testing.T) {
	db := testutils.SetupDB(t)

	ctx := context.Background()
	resolver := &graph.Resolver{
		DB:          db,
		WalletTable: "test_wallets",
	}

	mutation := resolver.Mutation()

	aAddress := "0xA000000000000000000000000000000000000000"
	smallAAddress := "0xa000000000000000000000000000000000000000" // lower and upper letters are treated the same

	// Clean and seed test data
	clearWallets(t, db)
	initWallet(t, db, aAddress, "10")

	// Transfer
	_, err := mutation.Transfer(ctx, aAddress, smallAAddress, "1")

	// Check if transfer throws error
	if err == nil {
		t.Fatal("Transfer with invalid amount did not throw error")
	}
	// Check error type
	if !strings.Contains(err.Error(), "sender and recipient addresses must be different") {
		t.Fatalf("Expected 'sender and recipient addresses must be different' error, got: %v", err)
	}

}

func TestValidateEthereumAddress(t *testing.T) {
	db := testutils.SetupDB(t)

	ctx := context.Background()
	resolver := &graph.Resolver{
		DB:          db,
		WalletTable: "test_wallets",
	}

	mutation := resolver.Mutation()

	aAddress := "0xA000000000000000000000000000000000000000"

	// Clean and seed test data
	clearWallets(t, db)
	initWallet(t, db, aAddress, "10")

	// Address is too short
	wrongAddress := "0xa00000000000000000000000000000000000000"
	_, err := mutation.Transfer(ctx, aAddress, wrongAddress, "1")
	// Check if transfer throws error
	if err == nil {
		t.Fatal("Transfer with invalid amount did not throw error")
	}
	// Check error type
	if !strings.Contains(err.Error(), "invalid Ethereum address format") {
		t.Fatalf("Expected 'invalid Ethereum address format' error, got: %v", err)
	}

	// Address does not start with '0x'
	wrongAddress = "00a000000000000000000000000000000000000000"
	_, err = mutation.Transfer(ctx, aAddress, wrongAddress, "1")
	// Check if transfer throws error
	if err == nil {
		t.Fatal("Transfer with invalid amount did not throw error")
	}
	// Check error type
	if !strings.Contains(err.Error(), "invalid Ethereum address format") {
		t.Fatalf("Expected 'invalid Ethereum address format' error, got: %v", err)
	}

	// Address has letters other than A-F
	wrongAddress = "0xG000000000000000000000000000000000000000"
	_, err = mutation.Transfer(ctx, aAddress, wrongAddress, "1")
	// Check if transfer throws error
	if err == nil {
		t.Fatal("Transfer with invalid amount did not throw error")
	}
	// Check error type
	if !strings.Contains(err.Error(), "invalid Ethereum address format") {
		t.Fatalf("Expected 'invalid Ethereum address format' error, got: %v", err)
	}

}

func TestCyclicTransfer(t *testing.T) {
	db := testutils.SetupDB(t)

	ctx := context.Background()
	resolver := &graph.Resolver{
		DB:          db,
		WalletTable: "test_wallets",
	}

	mutation := resolver.Mutation()

	aAddress := "0xA000000000000000000000000000000000000000"
	bAddress := "0xB000000000000000000000000000000000000000"
	cAddress := "0xC000000000000000000000000000000000000000"

	// Clean and seed test data
	clearWallets(t, db)
	initWallet(t, db, aAddress, "10")

	// A -> B Transfer
	amount := "10"
	fromAddress := aAddress
	toAddress := bAddress
	doTransfer(t, mutation, ctx, fromAddress, toAddress, amount)

	// B -> C Transfer
	fromAddress = bAddress
	toAddress = cAddress
	doTransfer(t, mutation, ctx, fromAddress, toAddress, amount)

	// C -> A Transfer
	fromAddress = cAddress
	toAddress = aAddress
	doTransfer(t, mutation, ctx, fromAddress, toAddress, amount)

	// Check balances
	expectedA := "10"
	expectedB := "0"
	expectedC := "0"

	assertBalance(t, db, expectedA, aAddress)
	assertBalance(t, db, expectedB, bAddress)
	assertBalance(t, db, expectedC, cAddress)

}

func TestRaceConditionSameWalletConcurrentTransfers(t *testing.T) {
	db := testutils.SetupDB(t)

	ctx := context.Background()
	resolver := &graph.Resolver{
		DB:          db,
		WalletTable: "test_wallets",
	}

	mutation := resolver.Mutation()

	aAddress := "0xA000000000000000000000000000000000000000"
	bAddress := "0xB000000000000000000000000000000000000000"
	cAddress := "0xC000000000000000000000000000000000000000"
	dAddress := "0xD000000000000000000000000000000000000000"

	// Clean and seed test data
	clearWallets(t, db)
	initWallet(t, db, aAddress, "10")
	initWallet(t, db, dAddress, "10")

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
		_, err := mutation.Transfer(ctx, aAddress, bAddress, "4")
		if err != nil && !strings.Contains(err.Error(), "insufficient balance") {
			t.Errorf("A -> B failed unexpectedly: %v", err)
		}
	}()

	// Transfer: A -> C (amount 7)
	// can fail due to insufficent balance
	go func() {
		defer wg.Done()
		<-start // barrier up
		_, err := mutation.Transfer(ctx, aAddress, cAddress, "7")
		if err != nil && !strings.Contains(err.Error(), "insufficient balance") {
			t.Errorf("A -> C failed unexpectedly: %v", err)
		}
	}()

	// Transfer: D -> A (amount 1)
	go func() {
		defer wg.Done()
		<-start // barrier up
		_, err := mutation.Transfer(ctx, dAddress, aAddress, "1")
		if err != nil {
			t.Errorf("D -> A failed unexpectedly: %v", err)
		}
	}()

	close(start) // bariers down

	// Wait for all to finish
	wg.Wait()

	// Check final balances
	aBalance := getBalance(t, db, aAddress)
	bBalance := getBalance(t, db, bAddress)
	cBalance := getBalance(t, db, cAddress)

	t.Logf("Final balances: A = %s, B = %s, C = %s", aBalance, bBalance, cBalance)

	// Convert balance string into decimal
	aDec, err := decimal.NewFromString(aBalance)
	if err != nil {
		t.Fatalf("Invalid decimal in DB balance for %s: %v", aAddress, err)
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
	db := testutils.SetupDB(t)

	ctx := context.Background()
	resolver := &graph.Resolver{
		DB:          db,
		WalletTable: "test_wallets",
	}

	mutation := resolver.Mutation()

	aAddress := "0xA000000000000000000000000000000000000000"
	bAddress := "0xB000000000000000000000000000000000000000"

	// Clean and seed test data
	clearWallets(t, db)
	initWallet(t, db, aAddress, "1000")
	initWallet(t, db, bAddress, "1000")

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
		fromAddress := aAddress
		toAddress := bAddress
		amount := "5"

		//  B -> A
		if i%2 == 1 {
			fromAddress, toAddress = bAddress, aAddress
			amount = "10"
		}

		// Transfer
		go func(from, to string, amount string) {
			defer wg.Done()
			<-start // barrier up

			doTransfer(t, mutation, ctx, fromAddress, toAddress, amount)
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

	assertBalance(t, db, expectedA, aAddress)
	assertBalance(t, db, expectedB, bAddress)
}
