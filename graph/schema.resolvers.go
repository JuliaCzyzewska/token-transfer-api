package graph

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hash/fnv"
	"math/big"
	"token_transfer/graph/model"

	"github.com/shopspring/decimal"
)

// Helpers
// Convert address to int64 using hash
func hashAddress(address string) int64 {
	h := fnv.New64()
	h.Write([]byte(address))
	return int64(h.Sum64())
}

// Add advisory locks on addresses
func (r *mutationResolver) lockWallets(tx *sql.Tx, fromAddress, toAddress string) error {
	senderHash := hashAddress(fromAddress)
	recipientHash := hashAddress(toAddress)

	// locks hashes always in the same order, to avoid deadlock
	if senderHash < recipientHash {
		if err := r.lockHashAddress(tx, senderHash); err != nil {
			return err
		}
		return r.lockHashAddress(tx, recipientHash)
	} else {
		if err := r.lockHashAddress(tx, recipientHash); err != nil {
			return err
		}
		return r.lockHashAddress(tx, senderHash)
	}
}

func (r *mutationResolver) lockHashAddress(tx *sql.Tx, hashAddressKey int64) error {
	_, err := tx.Exec("SELECT pg_advisory_xact_lock($1)", hashAddressKey)
	return err
}

// Add wallet with 0 tokens
func (r *mutationResolver) addWallet(tx *sql.Tx, address string) error {
	_, err := tx.Exec("INSERT INTO wallets (address, token_balance) VALUES ($1, 0)", address)
	return err
}

// Return token_balance as string
func (r *mutationResolver) getTokenBalance(tx *sql.Tx, address string) (string, error) {
	var balance string
	err := tx.QueryRow("SELECT token_balance FROM wallets WHERE address = $1", address).Scan(&balance)
	return balance, err
}

// Update balances; explicit cast amount from string to numeric
func (r *mutationResolver) updateBalances(tx *sql.Tx, fromAddress, toAddress string, amount string) error {
	_, err := tx.Exec(`UPDATE wallets SET token_balance = token_balance - $1::numeric WHERE address = $2`, amount, fromAddress)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE wallets SET token_balance = token_balance + $1::numeric WHERE address = $2`, amount, toAddress)
	return err
}

// Validate if token count checks the contraints of DB => NUMERIC(28, 18)
func validateTokenAmount(amount string) error {
	amountDecimal, err := decimal.NewFromString(amount)
	if err != nil {
		return fmt.Errorf("invalid decimal amount")
	}

	if amountDecimal.Cmp(decimal.Zero) <= 0 {
		return fmt.Errorf("amount must be greater than zero")
	}

	if amountDecimal.Exponent() < -18 {
		return fmt.Errorf("too many decimal places: max 18 allowed")
	}

	// Check if amount does not have more than 28 digits
	coeff := amountDecimal.Coefficient()
	totalDigits := len(coeff.String())
	if totalDigits > 28 {
		return fmt.Errorf("too many digits: max precision is 28")
	}
	return nil
}

// Resolver for the transfer field
func (r *mutationResolver) Transfer(ctx context.Context, fromAddress string, toAddress string, amount string) (string, error) {
	tx, err := r.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	// Validate amount
	if err := validateTokenAmount(amount); err != nil {
		return "", err
	}

	// Add advisory lock for server and recipient
	// If other transactions try to add lock, they will have to wait
	// until the end of transaction
	if err := r.lockWallets(tx, fromAddress, toAddress); err != nil {
		return "", err
	}

	// Get sender balance in string
	senderBalanceStr, err := r.getTokenBalance(tx, fromAddress)
	if err != nil {
		return "", err
	}

	// Parse sender balance and amount into big.Rat
	senderBalance := new(big.Rat)
	if _, ok := senderBalance.SetString(senderBalanceStr); !ok {
		return "", fmt.Errorf("invalid sender balance format in DB")
	}
	transferAmount := new(big.Rat)
	if _, ok := transferAmount.SetString(amount); !ok {
		return "", fmt.Errorf("invalid transfer amount format")
	}

	// Check balance of the sender
	if senderBalance.Cmp(transferAmount) < 0 {
		return "", fmt.Errorf("insufficient balance")
	}

	// Check if recipient wallet exists
	// If not - add it to DB
	_, err = r.getTokenBalance(tx, toAddress)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if err := r.addWallet(tx, toAddress); err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}

	// Update token balances
	if err := r.updateBalances(tx, fromAddress, toAddress, amount); err != nil {
		return "", err
	}

	// Commit
	if err := tx.Commit(); err != nil {
		return "", err
	}

	// Return new sender balance as a string
	newSenderBalance := new(big.Rat).Sub(senderBalance, transferAmount)
	return newSenderBalance.FloatString(18), nil
}

// Resolver for the wallet field
func (r *queryResolver) Wallet(ctx context.Context, address string) (*model.Wallet, error) {
	row := r.DB.QueryRow("SELECT address, token_balance FROM wallets WHERE address = $1", address)

	var wallet model.Wallet
	err := row.Scan(&wallet.Address, &wallet.Balance)
	if err != nil {
		return nil, err
	}

	return &wallet, nil
}

// Mutation returns MutationResolver implementation
func (r *Resolver) Mutation() MutationResolver { return &mutationResolver{r} }

// Query returns QueryResolver implementation
func (r *Resolver) Query() QueryResolver { return &queryResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
