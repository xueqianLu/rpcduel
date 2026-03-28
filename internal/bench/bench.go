package bench

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/xueqianLu/rpcduel/internal/dataset"
	"github.com/xueqianLu/rpcduel/pkg/rpc"
)

type Request struct {
	Method string
	Params []any
}

type Summary struct {
	Endpoint          string
	Requests          int
	Successes         int
	Failures          int
	RPS               float64
	P95               time.Duration
	P99               time.Duration
	ErrorDistribution map[string]int
}

func BuildRequests(file *dataset.File) []Request {
	requests := make([]Request, 0, len(file.Records)*2)
	for _, record := range file.Records {
		switch record.Type {
		case dataset.RecordTypeBlock:
			if record.Block == nil {
				continue
			}
			requests = append(requests, Request{
				Method: "eth_getBlockByNumber",
				Params: []any{rpc.HexBlockNumber(record.Block.Number), false},
			})
		case dataset.RecordTypeTransaction:
			if record.Transaction == nil {
				continue
			}
			requests = append(requests,
				Request{
					Method: "eth_getTransactionByHash",
					Params: []any{record.Transaction.Hash},
				},
				Request{
					Method: "eth_getTransactionReceipt",
					Params: []any{record.Transaction.Hash},
				},
			)
		case dataset.RecordTypeAddress:
			if record.Address == nil {
				continue
			}
			blockTag := rpc.HexBlockNumber(record.Address.FirstSeenBlock)
			requests = append(requests,
				Request{
					Method: "eth_getBalance",
					Params: []any{record.Address.Address, blockTag},
				},
				Request{
					Method: "eth_getTransactionCount",
					Params: []any{record.Address.Address, blockTag},
				},
			)
		}
	}
	return requests
}

func Run(ctx context.Context, provider *rpc.Provider, requests []Request, concurrency, total int) (Summary, error) {
	if len(requests) == 0 {
		return Summary{}, fmt.Errorf("no benchmark requests supplied")
	}
	if concurrency <= 0 {
		concurrency = 1
	}
	if total <= 0 {
		total = len(requests)
	}

	start := time.Now()
	jobs := make(chan int)
	results := make(chan callResult, concurrency)

	var workerGroup sync.WaitGroup
	for worker := 0; worker < concurrency; worker++ {
		workerGroup.Add(1)
		go func() {
			defer workerGroup.Done()
			for index := range jobs {
				request := requests[index%len(requests)]
				_, meta, err := provider.Call(ctx, request.Method, request.Params...)
				results <- callResult{
					latency:    meta.Duration,
					statusCode: meta.StatusCode,
					err:        err,
				}
			}
		}()
	}

	go func() {
		for index := 0; index < total; index++ {
			jobs <- index
		}
		close(jobs)
		workerGroup.Wait()
		close(results)
	}()

	latencies := make([]time.Duration, 0, total)
	summary := Summary{
		Endpoint:          provider.Target().Name,
		ErrorDistribution: map[string]int{},
	}

	for result := range results {
		summary.Requests++
		latencies = append(latencies, result.latency)
		if result.err == nil {
			summary.Successes++
			continue
		}

		summary.Failures++
		key := classifyError(result.statusCode, result.err)
		summary.ErrorDistribution[key]++
	}

	elapsed := time.Since(start)
	if elapsed > 0 {
		summary.RPS = float64(summary.Requests) / elapsed.Seconds()
	}
	summary.P95 = percentile(latencies, 95)
	summary.P99 = percentile(latencies, 99)

	return summary, nil
}

type callResult struct {
	latency    time.Duration
	statusCode int
	err        error
}

func classifyError(statusCode int, err error) string {
	var rpcErr *rpc.RPCError
	if errors.As(err, &rpcErr) {
		return fmt.Sprintf("rpc_%d", rpcErr.Code)
	}

	if statusCode > 0 && statusCode != http.StatusOK {
		return fmt.Sprintf("http_%d", statusCode)
	}

	return "transport"
}

func percentile(values []time.Duration, p int) time.Duration {
	if len(values) == 0 {
		return 0
	}

	sorted := append([]time.Duration(nil), values...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	index := (len(sorted)*p + 99) / 100
	if index <= 0 {
		index = 1
	}
	if index > len(sorted) {
		index = len(sorted)
	}
	return sorted[index-1]
}
