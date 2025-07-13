package graph_test

import (
	"testing"

	"token_transfer/graph/testutils"
)

func TestSomethingWithDB(t *testing.T) {
	db := testutils.SetupDB(t)

	// Twój test korzystający z db
	_ = db
}
