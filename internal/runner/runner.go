// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

// Package runner provides a concurrent worker pool for RPC requests.
package runner

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/xueqianLu/rpcduel/internal/rpc"
)

// Result holds the outcome of a single RPC call.
type Result struct {
	Endpoint  string
	Tag       string // scenario or other caller-supplied label; empty if unused
	Response  *rpc.Response
	Latency   time.Duration
	Err       error
	Timestamp time.Time // wall-clock time when the call completed
}

// Task is a single unit of work for the runner.
type Task struct {
	Endpoint string
	Tag      string // scenario or other caller-supplied label; empty if unused
	Method   string
	Params   []interface{}
}

// TaskGenerator produces the next task for a worker during duration-based runs.
type TaskGenerator func(workerID, iteration int) Task

// Run executes tasks concurrently using a worker pool.
// It sends results to the returned channel and closes it when done.
func Run(ctx context.Context, tasks []Task, concurrency int, timeout time.Duration) <-chan Result {
	results := make(chan Result, len(tasks))

	taskCh := make(chan Task, len(tasks))
	for _, t := range tasks {
		taskCh <- t
	}
	close(taskCh)

	var wg sync.WaitGroup
	workers := concurrency
	if workers <= 0 {
		workers = 1
	}
	if workers > len(tasks) {
		workers = len(tasks)
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			clients := make(map[string]*rpc.Client)
			for task := range taskCh {
				select {
				case <-ctx.Done():
					results <- Result{Endpoint: task.Endpoint, Tag: task.Tag, Err: ctx.Err()}
					continue
				default:
				}
				c := getClientCtx(ctx, clients, task.Endpoint, timeout)
				results <- callOnce(ctx, c, task.Endpoint, task.Tag, task.Method, task.Params)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}

// RunDuration runs tasks continuously for the given duration using a worker pool.
// Each worker picks endpoint(s) round-robin.
func RunDuration(ctx context.Context, endpoints []string, method string, params []interface{},
	concurrency int, duration time.Duration, timeout time.Duration) <-chan Result {

	results := make(chan Result, concurrency*2)

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		ep := endpoints[i%len(endpoints)]
		go func(endpoint string) {
			defer wg.Done()
			clients := make(map[string]*rpc.Client)
			c := getClientCtx(ctx, clients, endpoint, timeout)
			deadline := time.Now().Add(duration)
			for time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					return
				default:
				}
				results <- callOnce(ctx, c, endpoint, "", method, params)
			}
		}(ep)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}

// RunN runs exactly n requests across endpoints using a worker pool.
func RunN(ctx context.Context, endpoints []string, method string, params []interface{},
	concurrency, n int, timeout time.Duration) <-chan Result {

	results := make(chan Result, concurrency*2)

	taskCh := make(chan string, n)
	for i := 0; i < n; i++ {
		taskCh <- endpoints[i%len(endpoints)]
	}
	close(taskCh)

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			clients := make(map[string]*rpc.Client)
			for endpoint := range taskCh {
				select {
				case <-ctx.Done():
					results <- Result{Endpoint: endpoint, Err: ctx.Err()}
					continue
				default:
				}
				c := getClientCtx(ctx, clients, endpoint, timeout)
				results <- callOnce(ctx, c, endpoint, "", method, params)
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}

// PairResult holds paired results from two endpoints for the same request.
type PairResult struct {
	Left  Result
	Right Result
}

// RunPaired sends the same request to two endpoints concurrently and returns pairs.
func RunPaired(ctx context.Context, endpointA, endpointB string, method string, params []interface{},
	concurrency, n int, timeout time.Duration) <-chan PairResult {

	pairs := make(chan PairResult, concurrency*2)

	taskCh := make(chan struct{}, n)
	for i := 0; i < n; i++ {
		taskCh <- struct{}{}
	}
	close(taskCh)

	workers := concurrency
	if workers > n {
		workers = n
	}

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cClients := make(map[string]*rpc.Client)
			cA := getClientCtx(ctx, cClients, endpointA, timeout)
			cB := getClientCtx(ctx, cClients, endpointB, timeout)
			for range taskCh {
				select {
				case <-ctx.Done():
					pairs <- PairResult{
						Left:  Result{Endpoint: endpointA, Err: ctx.Err()},
						Right: Result{Endpoint: endpointB, Err: ctx.Err()},
					}
					continue
				default:
				}

				if err := waitRate(ctx); err != nil {
					pairs <- PairResult{
						Left:  Result{Endpoint: endpointA, Err: err, Timestamp: time.Now()},
						Right: Result{Endpoint: endpointB, Err: err, Timestamp: time.Now()},
					}
					continue
				}

				var (
					lResult, rResult Result
					innerWg          sync.WaitGroup
				)
				innerWg.Add(2)
				go func() {
					defer innerWg.Done()
					resp, lat, err := cA.Call(ctx, method, params)
					lResult = Result{Endpoint: endpointA, Response: resp, Latency: lat, Err: err, Timestamp: time.Now()}
				}()
				go func() {
					defer innerWg.Done()
					resp, lat, err := cB.Call(ctx, method, params)
					rResult = Result{Endpoint: endpointB, Response: resp, Latency: lat, Err: err, Timestamp: time.Now()}
				}()
				innerWg.Wait()
				pairs <- PairResult{Left: lResult, Right: rResult}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(pairs)
	}()

	return pairs
}

// PairResultFromDuration is like RunPaired but runs for a fixed duration.
func PairResultFromDuration(ctx context.Context, endpointA, endpointB string, method string, params []interface{},
	concurrency int, duration time.Duration, timeout time.Duration) <-chan PairResult {

	pairs := make(chan PairResult, concurrency*2)

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cClients := make(map[string]*rpc.Client)
			cA := getClientCtx(ctx, cClients, endpointA, timeout)
			cB := getClientCtx(ctx, cClients, endpointB, timeout)
			deadline := time.Now().Add(duration)
			for time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					return
				default:
				}
				if err := waitRate(ctx); err != nil {
					return
				}
				var (
					lResult, rResult Result
					innerWg          sync.WaitGroup
				)
				innerWg.Add(2)
				go func() {
					defer innerWg.Done()
					resp, lat, err := cA.Call(ctx, method, params)
					lResult = Result{Endpoint: endpointA, Response: resp, Latency: lat, Err: err, Timestamp: time.Now()}
				}()
				go func() {
					defer innerWg.Done()
					resp, lat, err := cB.Call(ctx, method, params)
					rResult = Result{Endpoint: endpointB, Response: resp, Latency: lat, Err: err, Timestamp: time.Now()}
				}()
				innerWg.Wait()

				pairs <- PairResult{Left: lResult, Right: rResult}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(pairs)
	}()

	return pairs
}

// ResultToJSON returns the JSON-encoded result of an RPC response.
func ResultToJSON(r *rpc.Response) json.RawMessage {
	if r == nil {
		return json.RawMessage("null")
	}
	return r.Result
}

// RunDurationFromTasks runs a pool of workers for the given duration, cycling
// through tasks repeatedly. It is used for duration-mode bench with an input file.
func RunDurationFromTasks(ctx context.Context, tasks []Task, concurrency int,
	duration time.Duration, timeout time.Duration) <-chan Result {

	results := make(chan Result, concurrency*2)
	if len(tasks) == 0 {
		close(results)
		return results
	}

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		// Stagger each worker's starting position through the task list so
		// that workers don't all start from index 0 and repeat the same
		// requests. The modulo handles the case where concurrency > len(tasks).
		startIdx := i % len(tasks)
		go func(idx int) {
			defer wg.Done()
			clients := make(map[string]*rpc.Client)
			deadline := time.Now().Add(duration)
			for time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					return
				default:
				}
				task := tasks[idx%len(tasks)]
				c := getClientCtx(ctx, clients, task.Endpoint, timeout)
				results <- callOnce(ctx, c, task.Endpoint, task.Tag, task.Method, task.Params)
				idx++
			}
		}(startIdx)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}

// RunDurationGenerated runs a pool of workers for the given duration and asks
// the caller to generate each task on demand. This is useful when requests
// should be sampled continuously rather than from a fixed pre-built pool.
func RunDurationGenerated(ctx context.Context, concurrency int,
	duration time.Duration, timeout time.Duration, gen TaskGenerator) <-chan Result {

	results := make(chan Result, concurrency*2)
	if concurrency <= 0 {
		concurrency = 1
	}

	var wg sync.WaitGroup
	for workerID := 0; workerID < concurrency; workerID++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			clients := make(map[string]*rpc.Client)
			deadline := time.Now().Add(duration)
			for iter := 0; time.Now().Before(deadline); iter++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				task := gen(id, iter)
				c := getClientCtx(ctx, clients, task.Endpoint, timeout)
				results <- callOnce(ctx, c, task.Endpoint, task.Tag, task.Method, task.Params)
			}
		}(workerID)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}

// callOnce is the shared per-request execution path used by every runner
// loop. It honors the rate limiter attached to ctx (if any), captures a
// completion timestamp, and produces a fully-populated Result.
func callOnce(ctx context.Context, c *rpc.Client, endpoint, tag, method string, params []interface{}) Result {
	if err := waitRate(ctx); err != nil {
		return Result{Endpoint: endpoint, Tag: tag, Err: err, Timestamp: time.Now()}
	}
	resp, lat, err := c.Call(ctx, method, params)
	return Result{
		Endpoint:  endpoint,
		Tag:       tag,
		Response:  resp,
		Latency:   lat,
		Err:       err,
		Timestamp: time.Now(),
	}
}

func getClientCtx(ctx context.Context, cache map[string]*rpc.Client, endpoint string, timeout time.Duration) *rpc.Client {
	if c := cache[endpoint]; c != nil {
		return c
	}
	opts, _ := ClientOptionsFromContext(ctx)
	opts.Timeout = timeout
	c := rpc.NewClientWithOptions(endpoint, opts)
	cache[endpoint] = c
	return c
}
