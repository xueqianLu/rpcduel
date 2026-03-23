// Package dataset defines the shared data structures for rpcduel test datasets
// and provides helpers for persisting them to / loading them from disk.
package dataset

import (
	"encoding/json"
	"fmt"
	"os"
)

// Meta holds metadata about a collected dataset.
type Meta struct {
	Chain       string `json:"chain"`
	Blockscout  string `json:"blockscout"`
	GeneratedAt string `json:"generated_at"`
}

// Account is a chain account with its observed transaction count.
type Account struct {
	Address string `json:"address"`
	TxCount int64  `json:"tx_count"`
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

// Save serialises ds to the JSON file at path (pretty-printed).
func Save(path string, ds *Dataset) error {
	data, err := json.MarshalIndent(ds, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal dataset: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write dataset %s: %w", path, err)
	}
	return nil
}

// Load reads a dataset from the JSON file at path.
func Load(path string) (*Dataset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read dataset %s: %w", path, err)
	}
	var ds Dataset
	if err := json.Unmarshal(data, &ds); err != nil {
		return nil, fmt.Errorf("parse dataset %s: %w", path, err)
	}
	return &ds, nil
}
