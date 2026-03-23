package dataset_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xueqianLu/rpcduel/internal/dataset"
)

func TestSaveLoad(t *testing.T) {
	ds := &dataset.Dataset{
		Meta: dataset.Meta{
			Chain: "testchain",
			RPC:   "http://rpc.example.com",
		},
		Range: dataset.Range{From: 100, To: 200},
		Accounts: []dataset.Account{
			{
				Address: "0xabc",
				TxCount: 42,
				Transactions: []dataset.Transaction{
					{Hash: "0xtx1", BlockNumber: 150, From: "0xabc", To: "0xdef"},
				},
			},
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

	if loaded.Meta.Chain != "testchain" {
		t.Errorf("chain: got %s", loaded.Meta.Chain)
	}
	if loaded.Meta.RPC != "http://rpc.example.com" {
		t.Errorf("rpc: got %s", loaded.Meta.RPC)
	}
	if loaded.Range.From != 100 || loaded.Range.To != 200 {
		t.Errorf("range: got %+v", loaded.Range)
	}
	if len(loaded.Accounts) != 1 || loaded.Accounts[0].Address != "0xabc" {
		t.Errorf("accounts: got %+v", loaded.Accounts)
	}
	if len(loaded.Accounts[0].Transactions) != 1 || loaded.Accounts[0].Transactions[0].Hash != "0xtx1" {
		t.Errorf("account transactions: got %+v", loaded.Accounts[0].Transactions)
	}
	if len(loaded.Transactions) != 1 || loaded.Transactions[0].Hash != "0xtx1" {
		t.Errorf("transactions: got %+v", loaded.Transactions)
	}
	if len(loaded.Blocks) != 1 || loaded.Blocks[0].Number != 150 {
		t.Errorf("blocks: got %+v", loaded.Blocks)
	}
}

func TestSave_SortsAccountsByTxCountDesc(t *testing.T) {
	ds := &dataset.Dataset{
		Accounts: []dataset.Account{
			{Address: "0xlow", TxCount: 1},
			{Address: "0xhigh", TxCount: 100},
			{Address: "0xmid", TxCount: 50},
		},
	}
	path := filepath.Join(t.TempDir(), "sorted.json")
	if err := dataset.Save(path, ds); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := dataset.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Accounts[0].Address != "0xhigh" {
		t.Errorf("expected 0xhigh first, got %s", loaded.Accounts[0].Address)
	}
	if loaded.Accounts[1].Address != "0xmid" {
		t.Errorf("expected 0xmid second, got %s", loaded.Accounts[1].Address)
	}
	if loaded.Accounts[2].Address != "0xlow" {
		t.Errorf("expected 0xlow third, got %s", loaded.Accounts[2].Address)
	}
}

func TestSave_SortsBlocksByNumberDesc(t *testing.T) {
	ds := &dataset.Dataset{
		Blocks: []dataset.Block{
			{Number: 100, TxCount: 1},
			{Number: 300, TxCount: 1},
			{Number: 200, TxCount: 1},
		},
	}
	path := filepath.Join(t.TempDir(), "sorted_blocks.json")
	if err := dataset.Save(path, ds); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := dataset.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Blocks[0].Number != 300 {
		t.Errorf("expected block 300 first, got %d", loaded.Blocks[0].Number)
	}
	if loaded.Blocks[1].Number != 200 {
		t.Errorf("expected block 200 second, got %d", loaded.Blocks[1].Number)
	}
	if loaded.Blocks[2].Number != 100 {
		t.Errorf("expected block 100 third, got %d", loaded.Blocks[2].Number)
	}
}

func TestSave_SortsTxsByBlockNumberAsc(t *testing.T) {
	ds := &dataset.Dataset{
		Transactions: []dataset.Transaction{
			{Hash: "0xtx3", BlockNumber: 300},
			{Hash: "0xtx1", BlockNumber: 100},
			{Hash: "0xtx2", BlockNumber: 200},
		},
	}
	path := filepath.Join(t.TempDir(), "sorted_txs.json")
	if err := dataset.Save(path, ds); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := dataset.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Transactions[0].BlockNumber != 100 {
		t.Errorf("expected tx with block 100 first, got %d", loaded.Transactions[0].BlockNumber)
	}
	if loaded.Transactions[1].BlockNumber != 200 {
		t.Errorf("expected tx with block 200 second, got %d", loaded.Transactions[1].BlockNumber)
	}
	if loaded.Transactions[2].BlockNumber != 300 {
		t.Errorf("expected tx with block 300 third, got %d", loaded.Transactions[2].BlockNumber)
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
