package testutil

import (
	"testing"

	badgerhold "github.com/timshannon/badgerhold/v4"
)

// NewTestDB creates an in-memory BadgerHold store for use in tests.
// The store is automatically closed via t.Cleanup when the test ends.
func NewTestDB(t *testing.T) *badgerhold.Store {
	t.Helper()
	opts := badgerhold.DefaultOptions
	opts.InMemory = true
	// Some BadgerDB versions require a non-empty Dir even in InMemory mode.
	opts.Dir = t.TempDir()
	opts.ValueDir = opts.Dir

	store, err := badgerhold.Open(opts)
	if err != nil {
		t.Fatalf("failed to open in-memory badgerhold: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}
