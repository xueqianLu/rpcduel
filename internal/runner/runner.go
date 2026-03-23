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
	Endpoint string
	Response *rpc.Response
	Latency  time.Duration
	Err      error
}

// Task is a single unit of work for the runner.
type Task struct {
	Endpoint string
	Method   string
	Params   []interface{}
}

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
			for task := range taskCh {
				select {
				case <-ctx.Done():
					results <- Result{Endpoint: task.Endpoint, Err: ctx.Err()}
					continue
				default:
				}
				c := rpc.NewClient(task.Endpoint, timeout)
				resp, lat, err := c.Call(ctx, task.Method, task.Params)
				results <- Result{
					Endpoint: task.Endpoint,
					Response: resp,
					Latency:  lat,
					Err:      err,
				}
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
			c := rpc.NewClient(endpoint, timeout)
			deadline := time.Now().Add(duration)
			for time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					return
				default:
				}
				resp, lat, err := c.Call(ctx, method, params)
				results <- Result{
					Endpoint: endpoint,
					Response: resp,
					Latency:  lat,
					Err:      err,
				}
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
			for endpoint := range taskCh {
				select {
				case <-ctx.Done():
					results <- Result{Endpoint: endpoint, Err: ctx.Err()}
					continue
				default:
				}
				c := rpc.NewClient(endpoint, timeout)
				resp, lat, err := c.Call(ctx, method, params)
				results <- Result{
					Endpoint: endpoint,
					Response: resp,
					Latency:  lat,
					Err:      err,
				}
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
			cA := rpc.NewClient(endpointA, timeout)
			cB := rpc.NewClient(endpointB, timeout)
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

				var (
					lResult, rResult Result
					innerWg          sync.WaitGroup
				)
				innerWg.Add(2)
				go func() {
					defer innerWg.Done()
					resp, lat, err := cA.Call(ctx, method, params)
					lResult = Result{Endpoint: endpointA, Response: resp, Latency: lat, Err: err}
				}()
				go func() {
					defer innerWg.Done()
					resp, lat, err := cB.Call(ctx, method, params)
					rResult = Result{Endpoint: endpointB, Response: resp, Latency: lat, Err: err}
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
			cA := rpc.NewClient(endpointA, timeout)
			cB := rpc.NewClient(endpointB, timeout)
			deadline := time.Now().Add(duration)
			for time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					return
				default:
				}
				var (
					lResult, rResult Result
					innerWg          sync.WaitGroup
				)
				innerWg.Add(2)
				go func() {
					defer innerWg.Done()
					resp, lat, err := cA.Call(ctx, method, params)
					lResult = Result{Endpoint: endpointA, Response: resp, Latency: lat, Err: err}
				}()
				go func() {
					defer innerWg.Done()
					resp, lat, err := cB.Call(ctx, method, params)
					rResult = Result{Endpoint: endpointB, Response: resp, Latency: lat, Err: err}
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
			deadline := time.Now().Add(duration)
			for time.Now().Before(deadline) {
				select {
				case <-ctx.Done():
					return
				default:
				}
				task := tasks[idx%len(tasks)]
				c := rpc.NewClient(task.Endpoint, timeout)
				resp, lat, err := c.Call(ctx, task.Method, task.Params)
				results <- Result{
					Endpoint: task.Endpoint,
					Response: resp,
					Latency:  lat,
					Err:      err,
				}
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
