package graph_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"token_transfer/graph"
	"token_transfer/graph/testutils"

	"github.com/shopspring/decimal"
)

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

func assertBalance(t *testing.T, expectedA, actualA string) {
	t.Helper()

	// Convert balance strings into decimals
	aDec, err := decimal.NewFromString(actualA)
	if err != nil {
		t.Fatalf("Invalid decimal in DB balance.")
	}

	expectedADec, err := decimal.NewFromString(expectedA)
	if err != nil {
		t.Fatalf("Invalid decimal in expected balance.")
	}

	if !aDec.Equal(expectedADec) {
		t.Errorf("Unexpected balance: got %s; want %s",
			aDec.String(), expectedADec.String())
	}

}

func TestWalletResolver(t *testing.T) {
	db := testutils.SetupDB(t)
	ctx := context.Background()
	resolver := &graph.Resolver{DB: db}
	qr := resolver.Query()

	// Clean and seed test data
	aAddress := "0xA000000000000000000000000000000000000000"
	aBalance := "1000"
	clearWallets(t, db)
	initWallet(t, db, aAddress, aBalance)

	wallet, err := qr.Wallet(ctx, aAddress)
	if err != nil {
		t.Fatalf("Expected no error but got: %v", err)
	}

	if wallet == nil {
		t.Fatal("Expected wallet, got nil")
	}

	if wallet.Address != aAddress {
		t.Errorf("Expected address %s, got %s", aAddress, wallet.Address)
	}

	assertBalance(t, aBalance, wallet.Balance)
}

func TestWalletResolver_NoWallet(t *testing.T) {
	db := testutils.SetupDB(t)
	ctx := context.Background()
	resolver := &graph.Resolver{DB: db}
	qr := resolver.Query()

	// Clean test data
	aAddress := "0xA000000000000000000000000000000000000000"
	clearWallets(t, db)

	_, err := qr.Wallet(ctx, aAddress)
	if err == nil {
		t.Fatal("Query about nonexistent wallet did not throw error")
	}

	// Check error type
	if !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("Expected 'no rows' error, got: %v", err)
	}

}
