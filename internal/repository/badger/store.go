package badger

import (
	"encoding/json"

	badgerhold "github.com/timshannon/badgerhold/v4"
)

// Open opens (or creates) a BadgerHold store at the given directory.
// When inMemory is true the dir parameter is used only as a temp path
// for BadgerDB internal files; no data is persisted to disk.
//
// All values are encoded with JSON (not Gob) for human-readability and
// forward-compatibility with future MongoDB / PostgreSQL migrations.
func Open(dir string, inMemory bool) (*badgerhold.Store, error) {
	opts := badgerhold.DefaultOptions
	opts.Encoder = json.Marshal
	opts.Decoder = json.Unmarshal
	if inMemory {
		opts.InMemory = true
		// InMemory mode requires Dir/ValueDir to be empty strings
	} else {
		opts.Dir = dir
		opts.ValueDir = dir
	}
	return badgerhold.Open(opts)
}
