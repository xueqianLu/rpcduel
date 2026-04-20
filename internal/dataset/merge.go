package dataset

import "sort"

// Merge combines two datasets into a single dataset and re-applies the
// supplied caps. The result preserves transactions deduplicated by hash,
// blocks deduplicated by number, and per-account transaction lists
// deduplicated by hash; transaction counts are recomputed from the merged
// per-account lists. The returned dataset adopts the union of both block
// ranges. Both inputs may be nil.
//
// Caps follow the same conventions as ChainScanner.Scan: zero or negative
// means "no cap" except for maxTxPerAccount where 0 means unlimited.
func Merge(a, b *Dataset, maxAccounts, maxTxs, maxBlocks, maxTxPerAccount int) *Dataset {
	out := &Dataset{}

	// Pick the metadata from whichever input has the newer GeneratedAt.
	switch {
	case a == nil && b == nil:
		return out
	case a == nil:
		out.Meta = b.Meta
		out.Range = b.Range
	case b == nil:
		out.Meta = a.Meta
		out.Range = a.Range
	default:
		out.Meta = b.Meta
		if a.Meta.GeneratedAt > b.Meta.GeneratedAt {
			out.Meta = a.Meta
		}
		out.Range = Range{
			From: minInt64(a.Range.From, b.Range.From),
			To:   maxInt64(a.Range.To, b.Range.To),
		}
	}

	// Merge blocks (dedupe by Number).
	seenBlock := make(map[int64]bool)
	addBlock := func(b Block) {
		if seenBlock[b.Number] {
			return
		}
		seenBlock[b.Number] = true
		out.Blocks = append(out.Blocks, b)
	}
	if a != nil {
		for _, blk := range a.Blocks {
			addBlock(blk)
		}
	}
	if b != nil {
		for _, blk := range b.Blocks {
			addBlock(blk)
		}
	}
	sort.Slice(out.Blocks, func(i, j int) bool { return out.Blocks[i].Number > out.Blocks[j].Number })
	if maxBlocks > 0 && len(out.Blocks) > maxBlocks {
		out.Blocks = out.Blocks[:maxBlocks]
	}

	// Merge top-level transactions (dedupe by Hash).
	seenTx := make(map[string]bool)
	addTx := func(t Transaction) {
		if t.Hash == "" || seenTx[t.Hash] {
			return
		}
		seenTx[t.Hash] = true
		out.Transactions = append(out.Transactions, t)
	}
	if a != nil {
		for _, t := range a.Transactions {
			addTx(t)
		}
	}
	if b != nil {
		for _, t := range b.Transactions {
			addTx(t)
		}
	}
	sort.Slice(out.Transactions, func(i, j int) bool {
		return out.Transactions[i].BlockNumber < out.Transactions[j].BlockNumber
	})
	if maxTxs > 0 && len(out.Transactions) > maxTxs {
		out.Transactions = out.Transactions[:maxTxs]
	}

	// Merge accounts: union of addresses, per-account txs deduped by hash.
	type acc struct {
		txs       []Transaction
		seen      map[string]bool
		legacyCnt int64
	}
	merged := make(map[string]*acc)
	visit := func(src []Account) {
		for _, srcAcc := range src {
			a, ok := merged[srcAcc.Address]
			if !ok {
				a = &acc{seen: make(map[string]bool)}
				merged[srcAcc.Address] = a
			}
			// Track the largest reported tx count we've seen so legacy
			// datasets without per-account tx lists still surface.
			if srcAcc.TxCount > a.legacyCnt {
				a.legacyCnt = srcAcc.TxCount
			}
			for _, t := range srcAcc.Transactions {
				if t.Hash == "" || a.seen[t.Hash] {
					continue
				}
				a.seen[t.Hash] = true
				a.txs = append(a.txs, t)
			}
		}
	}
	if a != nil {
		visit(a.Accounts)
	}
	if b != nil {
		visit(b.Accounts)
	}
	for addr, m := range merged {
		sort.Slice(m.txs, func(i, j int) bool { return m.txs[i].BlockNumber < m.txs[j].BlockNumber })
		if maxTxPerAccount > 0 && len(m.txs) > maxTxPerAccount {
			m.txs = m.txs[:maxTxPerAccount]
		}
		count := int64(len(m.txs))
		if count == 0 {
			count = m.legacyCnt
		}
		out.Accounts = append(out.Accounts, Account{
			Address:      addr,
			TxCount:      count,
			Transactions: m.txs,
		})
	}
	sort.Slice(out.Accounts, func(i, j int) bool { return out.Accounts[i].TxCount > out.Accounts[j].TxCount })
	if maxAccounts > 0 && len(out.Accounts) > maxAccounts {
		out.Accounts = out.Accounts[:maxAccounts]
	}

	return out
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
