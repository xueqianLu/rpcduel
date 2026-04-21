package dataset

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func sampleDataset() *Dataset {
	return &Dataset{
		Meta:  Meta{SchemaVersion: 1, Chain: "ethereum", RPC: "http://x", GeneratedAt: "2024-01-01T00:00:00Z"},
		Range: Range{From: 100, To: 200},
		Accounts: []Account{
			{Address: "0xa", TxCount: 50},
			{Address: "0xb", TxCount: 7},
			{Address: "0xc", TxCount: 1},
			{Address: "0xd", TxCount: 1},
		},
		Transactions: []Transaction{
			{Hash: "0x1", BlockNumber: 100, From: "0xa", To: "0xb"},
			{Hash: "0x2", BlockNumber: 110, From: "0xa", To: "0xc"},
			{Hash: "0x2", BlockNumber: 110, From: "0xa", To: "0xc"}, // dup hash
		},
		Blocks: []Block{
			{Number: 100, TxCount: 1},
			{Number: 110, TxCount: 2},
		},
	}
}

func TestInspectCounts(t *testing.T) {
	s := Inspect("ds.json", sampleDataset())
	if s.Accounts != 4 || s.Transactions != 3 || s.Blocks != 2 {
		t.Fatalf("counts wrong: %+v", s)
	}
	if s.UniqueTxHashes != 2 {
		t.Errorf("unique tx hashes = %d, want 2", s.UniqueTxHashes)
	}
	if s.BlockSpan != 101 {
		t.Errorf("block span = %d, want 101", s.BlockSpan)
	}
	if len(s.TopAccounts) != 4 || s.TopAccounts[0].Address != "0xa" {
		t.Errorf("top accounts wrong: %+v", s.TopAccounts)
	}
	// Buckets: 1->2 (0xc, 0xd), 2-5->0, 6-20->1 (0xb), 21-100->1 (0xa)
	want := map[int64]int{1: 2, 6: 1, 21: 1}
	for _, b := range s.TxPerAccountHist {
		if w, ok := want[b.Min]; ok && b.Count != w {
			t.Errorf("bucket %d-%d count = %d, want %d", b.Min, b.Max, b.Count, w)
		}
	}
	// Total estimated calls = 4+4+3+3+2+2+3+2 = 23
	if s.EstimatedTotal != 23 {
		t.Errorf("estimated total = %d, want 23", s.EstimatedTotal)
	}
}

func TestPrintStatsTextHasHeaders(t *testing.T) {
	var buf bytes.Buffer
	PrintStats(&buf, Inspect("ds.json", sampleDataset()))
	out := buf.String()
	for _, want := range []string{"Dataset: ds.json", "Top accounts", "tx_per_account", "Estimated RPC"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestPrintStatsJSONRoundtrip(t *testing.T) {
	var buf bytes.Buffer
	if err := PrintStatsJSON(&buf, Inspect("ds.json", sampleDataset())); err != nil {
		t.Fatal(err)
	}
	var got Stats
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}
	if got.Accounts != 4 {
		t.Errorf("round-trip lost data: %+v", got)
	}
}
