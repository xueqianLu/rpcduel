package dataset

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/xueqianLu/rpcduel/pkg/rpc"
)

const (
	RecordTypeBlock       = "block"
	RecordTypeTransaction = "transaction"
	RecordTypeAddress     = "address"
)

type Metadata struct {
	GeneratedAt string `json:"generated_at"`
	Source      string `json:"source"`
	FromBlock   uint64 `json:"from_block"`
	ToBlock     uint64 `json:"to_block"`
}

type File struct {
	Meta    Metadata `json:"meta"`
	Records []Record `json:"records"`
}

type Record struct {
	Type        string           `json:"type"`
	Block       *BlockRecord     `json:"block,omitempty"`
	Transaction *TransactionData `json:"transaction,omitempty"`
	Address     *AddressData     `json:"address,omitempty"`
}

type BlockRecord struct {
	Number     uint64 `json:"number"`
	Hash       string `json:"hash"`
	ParentHash string `json:"parent_hash"`
	Timestamp  uint64 `json:"timestamp"`
	Miner      string `json:"miner,omitempty"`
	TxCount    int    `json:"tx_count"`
}

type TransactionData struct {
	Hash        string `json:"hash"`
	BlockNumber uint64 `json:"block_number"`
	From        string `json:"from,omitempty"`
	To          string `json:"to,omitempty"`
}

type AddressData struct {
	Address        string `json:"address"`
	FirstSeenBlock uint64 `json:"first_seen_block"`
}

type Summary struct {
	Blocks       int
	Transactions int
	Addresses    int
}

type CollectorOption func(*Collector)

type Collector struct {
	provider    *rpc.Provider
	concurrency int
}

func NewCollector(provider *rpc.Provider, options ...CollectorOption) *Collector {
	collector := &Collector{
		provider:    provider,
		concurrency: defaultCollectorConcurrency(),
	}
	for _, option := range options {
		option(collector)
	}
	if collector.concurrency <= 0 {
		collector.concurrency = 1
	}
	return collector
}

func WithConcurrency(concurrency int) CollectorOption {
	return func(collector *Collector) {
		collector.concurrency = concurrency
	}
}

func (c *Collector) Collect(ctx context.Context, from, to uint64, output io.Writer) (Summary, error) {
	if from > to {
		return Summary{}, fmt.Errorf("invalid range: from=%d is greater than to=%d", from, to)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	writer := newStreamWriter(output)
	if err := writer.Begin(Metadata{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Source:      c.provider.Target().Name,
		FromBlock:   from,
		ToBlock:     to,
	}); err != nil {
		return Summary{}, err
	}

	summary := Summary{}
	seenTransactions := make(map[string]struct{})
	seenAddresses := make(map[string]struct{})
	concurrency := min(c.concurrency, int(to-from+1))
	if concurrency <= 0 {
		concurrency = 1
	}

	jobs := make(chan uint64)
	results := make(chan blockFetchResult, concurrency)

	var workerGroup sync.WaitGroup
	for worker := 0; worker < concurrency; worker++ {
		workerGroup.Add(1)
		go func() {
			defer workerGroup.Done()
			for blockNumber := range jobs {
				result := c.fetchBlock(ctx, blockNumber)
				select {
				case results <- result:
				case <-ctx.Done():
					return
				}
				if result.err != nil {
					return
				}
			}
		}()
	}

	go func() {
		defer close(jobs)
		for blockNumber := from; ; blockNumber++ {
			select {
			case jobs <- blockNumber:
			case <-ctx.Done():
				return
			}
			if blockNumber == to {
				return
			}
		}
	}()

	go func() {
		workerGroup.Wait()
		close(results)
	}()

	pending := make(map[uint64]blockFetchResult, concurrency)
	nextBlock := from
	var firstErr error

	for result := range results {
		if result.err != nil && firstErr == nil {
			firstErr = result.err
			cancel()
		}

		pending[result.blockNumber] = result
		for {
			current, ok := pending[nextBlock]
			if !ok {
				break
			}
			delete(pending, nextBlock)
			if current.err != nil {
				break
			}
			if current.blockRecord != nil {
				if err := writer.WriteRecord(Record{
					Type:  RecordTypeBlock,
					Block: current.blockRecord,
				}); err != nil {
					cancel()
					return summary, err
				}
				summary.Blocks++
			}

			for _, transaction := range current.transactions {
				if _, ok := seenTransactions[transactionKey(transaction.Hash)]; ok {
					continue
				}
				seenTransactions[transactionKey(transaction.Hash)] = struct{}{}
				if err := writer.WriteRecord(Record{
					Type:        RecordTypeTransaction,
					Transaction: transaction,
				}); err != nil {
					cancel()
					return summary, err
				}
				summary.Transactions++
			}

			for _, address := range current.addresses {
				if _, ok := seenAddresses[addressKey(address.Address)]; ok {
					continue
				}
				seenAddresses[addressKey(address.Address)] = struct{}{}
				if err := writer.WriteRecord(Record{
					Type:    RecordTypeAddress,
					Address: address,
				}); err != nil {
					cancel()
					return summary, err
				}
				summary.Addresses++
			}

			if nextBlock == to {
				break
			}
			nextBlock++
		}
	}

	if firstErr != nil {
		_ = writer.Close()
		return summary, firstErr
	}

	if err := writer.Close(); err != nil {
		return summary, err
	}
	return summary, nil
}

type blockFetchResult struct {
	blockNumber  uint64
	blockRecord  *BlockRecord
	transactions []*TransactionData
	addresses    []*AddressData
	err          error
}

func (c *Collector) fetchBlock(ctx context.Context, blockNumber uint64) blockFetchResult {
	response, _, err := c.provider.BlockByNumber(ctx, blockNumber, true)
	if err != nil {
		return blockFetchResult{
			blockNumber: blockNumber,
			err:         fmt.Errorf("fetch block %d: %w", blockNumber, err),
		}
	}

	blockRecord, transactions, addresses, err := decodeBlock(response.Result)
	if err != nil {
		return blockFetchResult{
			blockNumber: blockNumber,
			err:         fmt.Errorf("decode block %d: %w", blockNumber, err),
		}
	}

	return blockFetchResult{
		blockNumber:  blockNumber,
		blockRecord:  blockRecord,
		transactions: transactions,
		addresses:    addresses,
	}
}

func Load(path string) (*File, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read dataset %s: %w", path, err)
	}

	var file File
	if err := json.Unmarshal(bytes, &file); err != nil {
		return nil, fmt.Errorf("decode dataset %s: %w", path, err)
	}
	return &file, nil
}

func decodeBlock(raw json.RawMessage) (*BlockRecord, []*TransactionData, []*AddressData, error) {
	if trimmed := strings.TrimSpace(string(raw)); trimmed == "" || trimmed == "null" {
		return nil, nil, nil, nil
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, nil, nil, err
	}

	number, err := uint64Field(payload["number"])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("decode block number: %w", err)
	}
	timestamp, err := uint64Field(payload["timestamp"])
	if err != nil {
		return nil, nil, nil, fmt.Errorf("decode block timestamp: %w", err)
	}

	record := &BlockRecord{
		Number:     number,
		Hash:       stringField(payload["hash"]),
		ParentHash: stringField(payload["parentHash"]),
		Timestamp:  timestamp,
		Miner:      stringField(payload["miner"]),
	}

	addresses := make([]*AddressData, 0, 8)
	addAddress := func(address string) {
		address = strings.TrimSpace(address)
		if address == "" || address == "0x0000000000000000000000000000000000000000" {
			return
		}
		addresses = append(addresses, &AddressData{
			Address:        address,
			FirstSeenBlock: number,
		})
	}

	addAddress(record.Miner)

	rawTransactions, _ := payload["transactions"].([]any)
	record.TxCount = len(rawTransactions)

	transactions := make([]*TransactionData, 0, len(rawTransactions))
	for _, item := range rawTransactions {
		switch typed := item.(type) {
		case string:
			transactions = append(transactions, &TransactionData{
				Hash:        typed,
				BlockNumber: number,
			})
		case map[string]any:
			transactionBlock, err := uint64FieldFallback(typed["blockNumber"], number)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("decode transaction block number: %w", err)
			}
			tx := &TransactionData{
				Hash:        stringField(typed["hash"]),
				BlockNumber: transactionBlock,
				From:        stringField(typed["from"]),
				To:          stringField(typed["to"]),
			}
			transactions = append(transactions, tx)
			addAddress(tx.From)
			addAddress(tx.To)
		}
	}

	return record, transactions, addresses, nil
}

type streamWriter struct {
	writer io.Writer
	enc    *json.Encoder
	first  bool
}

func newStreamWriter(writer io.Writer) *streamWriter {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	return &streamWriter{
		writer: writer,
		enc:    encoder,
		first:  true,
	}
}

func (w *streamWriter) Begin(meta Metadata) error {
	if _, err := io.WriteString(w.writer, "{\"meta\":"); err != nil {
		return fmt.Errorf("write dataset prefix: %w", err)
	}
	if err := w.enc.Encode(meta); err != nil {
		return fmt.Errorf("encode dataset metadata: %w", err)
	}
	if _, err := io.WriteString(w.writer, ",\"records\":["); err != nil {
		return fmt.Errorf("open dataset records: %w", err)
	}
	return nil
}

func (w *streamWriter) WriteRecord(record Record) error {
	if !w.first {
		if _, err := io.WriteString(w.writer, ","); err != nil {
			return fmt.Errorf("write dataset separator: %w", err)
		}
	}
	w.first = false

	if err := w.enc.Encode(record); err != nil {
		return fmt.Errorf("encode dataset record: %w", err)
	}
	return nil
}

func (w *streamWriter) Close() error {
	if _, err := io.WriteString(w.writer, "]}"); err != nil {
		return fmt.Errorf("close dataset stream: %w", err)
	}
	return nil
}

func stringField(value any) string {
	typed, _ := value.(string)
	return typed
}

func uint64Field(value any) (uint64, error) {
	text := stringField(value)
	if text == "" {
		return 0, fmt.Errorf("missing quantity")
	}
	return rpc.ParseQuantityUint64(text)
}

func uint64FieldFallback(value any, fallback uint64) (uint64, error) {
	text := stringField(value)
	if text == "" {
		return fallback, nil
	}
	return rpc.ParseQuantityUint64(text)
}

func transactionKey(hash string) string {
	return strings.ToLower(strings.TrimSpace(hash))
}

func addressKey(address string) string {
	return strings.ToLower(strings.TrimSpace(address))
}

func defaultCollectorConcurrency() int {
	concurrency := runtime.GOMAXPROCS(0)
	if concurrency < 1 {
		return 1
	}
	if concurrency > 8 {
		return 8
	}
	return concurrency
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}
