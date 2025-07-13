package graph_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"token_transfer/graph"
	"token_transfer/graph/tests/testutils"
)

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

	assertBalance(t, db, wallet.Balance, aAddress)
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
