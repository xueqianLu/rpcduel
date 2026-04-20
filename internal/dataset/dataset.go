// Package dataset defines the shared data structures for rpcduel test datasets
// and provides helpers for persisting them to / loading them from disk.
package dataset

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

// SchemaVersion is the current dataset schema version. Files written by
// Save embed this value in Meta.SchemaVersion. Load accepts datasets with
// either no version (legacy v0) or a version <= SchemaVersion; newer files
// are rejected with a clear error so users can upgrade rpcduel.
const SchemaVersion = 1

// Meta holds metadata about a collected dataset.
type Meta struct {
	// SchemaVersion is the dataset file format version. Files written by
	// older versions of rpcduel may omit this field (treated as 0).
	SchemaVersion int    `json:"schema_version,omitempty"`
	Chain         string `json:"chain"`
	RPC           string `json:"rpc"`
	GeneratedAt   string `json:"generated_at"`
}

// Account is a chain account with its observed transaction count and the
// transactions it participated in during the scan range.
type Account struct {
	Address      string        `json:"address"`
	TxCount      int64         `json:"tx_count"`
	Transactions []Transaction `json:"transactions,omitempty"`
}

// Transaction is a minimal on-chain transaction record.
type Transaction struct {
	Hash        string `json:"hash"`
	BlockNumber int64  `json:"block_number"`
	From        string `json:"from"`
	To          string `json:"to"`
}

// Block is a minimal on-chain block record.
type Block struct {
	Number  int64 `json:"number"`
	TxCount int   `json:"tx_count"`
}

// Range describes an inclusive block range.
type Range struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

// Dataset is the top-level structure written to / read from a dataset file.
type Dataset struct {
	Meta         Meta          `json:"meta"`
	Range        Range         `json:"range"`
	Accounts     []Account     `json:"accounts"`
	Transactions []Transaction `json:"transactions"`
	Blocks       []Block       `json:"blocks"`
}

// Save serializes ds to the JSON file at path (pretty-printed).
// Accounts are sorted by tx_count descending, blocks by number descending,
// and transactions by block_number ascending.
func Save(path string, ds *Dataset) error {
	// Sort accounts by TxCount descending.
	sort.Slice(ds.Accounts, func(i, j int) bool {
		return ds.Accounts[i].TxCount > ds.Accounts[j].TxCount
	})
	// Sort blocks by Number descending (newest first).
	sort.Slice(ds.Blocks, func(i, j int) bool {
		return ds.Blocks[i].Number > ds.Blocks[j].Number
	})
	// Sort transactions by BlockNumber ascending (chronological order).
	sort.Slice(ds.Transactions, func(i, j int) bool {
		return ds.Transactions[i].BlockNumber < ds.Transactions[j].BlockNumber
	})

	// Stamp the current schema version on every Save.
	ds.Meta.SchemaVersion = SchemaVersion

	data, err := json.MarshalIndent(ds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal dataset: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write dataset %s: %w", path, err)
	}
	return nil
}

// Load reads a dataset from the JSON file at path. Datasets written by older
// rpcduel versions (no schema_version field) are accepted as legacy v0.
// Datasets carrying a schema_version newer than the one this binary
// understands are rejected.
func Load(path string) (*Dataset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read dataset %s: %w", path, err)
	}
	var ds Dataset
	if err := json.Unmarshal(data, &ds); err != nil {
		return nil, fmt.Errorf("parse dataset %s: %w", path, err)
	}
	if ds.Meta.SchemaVersion > SchemaVersion {
		return nil, fmt.Errorf(
			"dataset %s has schema_version=%d, which is newer than this binary supports (max %d); please upgrade rpcduel",
			path, ds.Meta.SchemaVersion, SchemaVersion)
	}
	return &ds, nil
}
