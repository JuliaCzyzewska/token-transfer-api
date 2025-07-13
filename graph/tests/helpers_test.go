package graph_test

import (
	"context"
	"database/sql"
	"testing"

	"token_transfer/graph"

	"github.com/shopspring/decimal"
)

func initWallet(t *testing.T, db *sql.DB, address string, balance string) {
	t.Helper()
	_, err := db.Exec("INSERT INTO test_wallets (address, token_balance) VALUES ($1, $2::numeric)", address, balance)
	if err != nil {
		t.Fatalf("Failed to insert wallet %s: %v", address, err)
	}
}

func clearWallets(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec("DELETE FROM test_wallets")
	if err != nil {
		t.Fatalf("Failed to clear wallets: %v", err)
	}
}

func getBalance(t *testing.T, db *sql.DB, address string) string {
	t.Helper()
	var balance string
	err := db.QueryRow("SELECT token_balance FROM test_wallets WHERE address = $1", address).Scan(&balance)
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

func doTransfer(t *testing.T, resolver graph.MutationResolver, ctx context.Context, fromAddress, toAddress, amount string) {
	t.Helper()

	_, err := resolver.Transfer(ctx, fromAddress, toAddress, amount)
	if err != nil {
		t.Errorf("Transfer %s → %s failed: %v", fromAddress, toAddress, err)
	}
}
