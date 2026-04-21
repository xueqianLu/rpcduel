// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package dataset

import "testing"

func TestMerge_DedupesAndCaps(t *testing.T) {
	a := &Dataset{
		Meta:  Meta{Chain: "eth", GeneratedAt: "2024-01-01T00:00:00Z"},
		Range: Range{From: 100, To: 200},
		Blocks: []Block{
			{Number: 200, TxCount: 1},
			{Number: 150, TxCount: 2},
		},
		Transactions: []Transaction{
			{Hash: "0xa", BlockNumber: 150, From: "0x1", To: "0x2"},
			{Hash: "0xb", BlockNumber: 200, From: "0x1", To: "0x3"},
		},
		Accounts: []Account{
			{Address: "0x1", TxCount: 2, Transactions: []Transaction{
				{Hash: "0xa", BlockNumber: 150}, {Hash: "0xb", BlockNumber: 200},
			}},
			{Address: "0x2", TxCount: 1, Transactions: []Transaction{
				{Hash: "0xa", BlockNumber: 150},
			}},
		},
	}
	b := &Dataset{
		Meta:  Meta{Chain: "eth", GeneratedAt: "2024-02-01T00:00:00Z"},
		Range: Range{From: 201, To: 300},
		Blocks: []Block{
			{Number: 300, TxCount: 1},
			{Number: 200, TxCount: 1}, // duplicate of a
			{Number: 250, TxCount: 1},
		},
		Transactions: []Transaction{
			{Hash: "0xc", BlockNumber: 250, From: "0x1", To: "0x4"},
			{Hash: "0xb", BlockNumber: 200, From: "0x1", To: "0x3"}, // dup
		},
		Accounts: []Account{
			{Address: "0x1", TxCount: 1, Transactions: []Transaction{
				{Hash: "0xc", BlockNumber: 250},
				{Hash: "0xb", BlockNumber: 200}, // dup
			}},
			{Address: "0x4", TxCount: 1, Transactions: []Transaction{
				{Hash: "0xc", BlockNumber: 250},
			}},
		},
	}

	out := Merge(a, b, 0, 0, 0, 0)

	if len(out.Blocks) != 4 {
		t.Errorf("blocks: got %d want 4 (deduped)", len(out.Blocks))
	}
	if out.Blocks[0].Number != 300 {
		t.Errorf("blocks not sorted desc: %+v", out.Blocks)
	}
	if len(out.Transactions) != 3 {
		t.Errorf("transactions: got %d want 3 (deduped)", len(out.Transactions))
	}
	if out.Range.From != 100 || out.Range.To != 300 {
		t.Errorf("range: got %+v want {100,300}", out.Range)
	}
	// 0x1 should have 3 unique txs, ranked first.
	if out.Accounts[0].Address != "0x1" || out.Accounts[0].TxCount != 3 {
		t.Errorf("top account: got %+v want 0x1 with 3 txs", out.Accounts[0])
	}
	if out.Meta.GeneratedAt != "2024-02-01T00:00:00Z" {
		t.Errorf("meta should pick newer GeneratedAt, got %q", out.Meta.GeneratedAt)
	}
}

func TestMerge_AppliesCaps(t *testing.T) {
	a := &Dataset{
		Blocks: []Block{{Number: 1}, {Number: 2}, {Number: 3}},
		Transactions: []Transaction{
			{Hash: "0xa", BlockNumber: 1},
			{Hash: "0xb", BlockNumber: 2},
		},
		Accounts: []Account{
			{Address: "0x1", TxCount: 5, Transactions: []Transaction{
				{Hash: "0xa", BlockNumber: 1},
				{Hash: "0xb", BlockNumber: 2},
				{Hash: "0xc", BlockNumber: 3},
			}},
			{Address: "0x2", TxCount: 1},
		},
	}
	out := Merge(a, nil, 1, 1, 2, 1)
	if len(out.Blocks) != 2 {
		t.Errorf("maxBlocks=2 not enforced: %d", len(out.Blocks))
	}
	if len(out.Transactions) != 1 {
		t.Errorf("maxTxs=1 not enforced: %d", len(out.Transactions))
	}
	if len(out.Accounts) != 1 {
		t.Errorf("maxAccounts=1 not enforced: %d", len(out.Accounts))
	}
	if len(out.Accounts[0].Transactions) != 1 {
		t.Errorf("maxTxPerAccount=1 not enforced: %d", len(out.Accounts[0].Transactions))
	}
}

func TestMerge_NilInputs(t *testing.T) {
	if got := Merge(nil, nil, 0, 0, 0, 0); got == nil {
		t.Fatal("expected non-nil empty dataset")
	}
}
