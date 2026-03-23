package dataset_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xueqianLu/rpcduel/internal/dataset"
)

func TestSaveLoad(t *testing.T) {
	ds := &dataset.Dataset{
		Chain: "testchain",
		Range: dataset.Range{From: 100, To: 200},
		Accounts: []dataset.Account{
			{Address: "0xabc", TxCount: 42},
		},
		Transactions: []dataset.Transaction{
			{Hash: "0xtx1", BlockNumber: 150, From: "0xabc", To: "0xdef"},
		},
		Blocks: []dataset.Block{
			{Number: 150, TxCount: 5},
		},
	}

	path := filepath.Join(t.TempDir(), "test_dataset.json")
	if err := dataset.Save(path, ds); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := dataset.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Chain != "testchain" {
		t.Errorf("chain: got %s", loaded.Chain)
	}
	if loaded.Range.From != 100 || loaded.Range.To != 200 {
		t.Errorf("range: got %+v", loaded.Range)
	}
	if len(loaded.Accounts) != 1 || loaded.Accounts[0].Address != "0xabc" {
		t.Errorf("accounts: got %+v", loaded.Accounts)
	}
	if len(loaded.Transactions) != 1 || loaded.Transactions[0].Hash != "0xtx1" {
		t.Errorf("transactions: got %+v", loaded.Transactions)
	}
	if len(loaded.Blocks) != 1 || loaded.Blocks[0].Number != 150 {
		t.Errorf("blocks: got %+v", loaded.Blocks)
	}
}

func TestLoad_NotFound(t *testing.T) {
	_, err := dataset.Load("/nonexistent/path/dataset.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := dataset.Load(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
