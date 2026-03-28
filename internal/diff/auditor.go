package diff

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"

	"github.com/xueqianLu/rpcduel/pkg/rpc"
)

type Endpoint struct {
	Target   rpc.Target
	Provider *rpc.Provider
}

type Finding struct {
	Endpoint    string       `json:"endpoint"`
	Method      string       `json:"method"`
	Params      []any        `json:"params"`
	Error       string       `json:"error,omitempty"`
	Differences []Difference `json:"differences,omitempty"`
}

type Report struct {
	Baseline string    `json:"baseline"`
	Targets  []string  `json:"targets"`
	From     uint64    `json:"from"`
	To       uint64    `json:"to"`
	Checks   int       `json:"checks"`
	Findings []Finding `json:"findings"`
}

type AuditorOption func(*Auditor)

type Auditor struct {
	baseline    Endpoint
	peers       []Endpoint
	options     Options
	concurrency int
}

func NewAuditor(baseline Endpoint, peers []Endpoint, options Options, auditorOptions ...AuditorOption) *Auditor {
	auditor := &Auditor{
		baseline:    baseline,
		peers:       peers,
		options:     options,
		concurrency: defaultAuditorConcurrency(),
	}
	for _, option := range auditorOptions {
		option(auditor)
	}
	if auditor.concurrency <= 0 {
		auditor.concurrency = 1
	}
	return auditor
}

func WithConcurrency(concurrency int) AuditorOption {
	return func(auditor *Auditor) {
		auditor.concurrency = concurrency
	}
}

func (a *Auditor) AuditBlockRange(ctx context.Context, from, to uint64) (*Report, error) {
	if from > to {
		return nil, fmt.Errorf("invalid block range: from=%d is greater than to=%d", from, to)
	}

	report := &Report{
		Baseline: a.baseline.Target.Name,
		From:     from,
		To:       to,
	}
	for _, peer := range a.peers {
		report.Targets = append(report.Targets, peer.Target.Name)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan uint64)
	concurrency := minInt(a.concurrency, int(to-from+1))
	if concurrency <= 0 {
		concurrency = 1
	}
	results := make(chan blockAuditResult, concurrency)

	var workerGroup sync.WaitGroup
	for worker := 0; worker < concurrency; worker++ {
		workerGroup.Add(1)
		go func() {
			defer workerGroup.Done()
			for blockNumber := range jobs {
				result := a.auditBlock(ctx, blockNumber)
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

	var firstErr error
	for result := range results {
		if result.err != nil {
			if firstErr == nil {
				firstErr = result.err
				cancel()
			}
			continue
		}
		report.Checks += result.checks
		report.Findings = append(report.Findings, result.findings...)
	}

	if firstErr != nil {
		return nil, firstErr
	}

	sortFindings(report.Findings)
	return report, nil
}

type blockAuditResult struct {
	blockNumber uint64
	checks      int
	findings    []Finding
	err         error
}

type compareCallResult struct {
	checks   int
	results  map[string]json.RawMessage
	findings []Finding
	err      error
}

func (a *Auditor) auditBlock(ctx context.Context, blockNumber uint64) blockAuditResult {
	if err := ctx.Err(); err != nil {
		return blockAuditResult{blockNumber: blockNumber, err: err}
	}

	var findings []Finding
	checks := 0
	blockTag := rpc.HexBlockNumber(blockNumber)

	blockResults := a.compareCall(ctx, "eth_getBlockByNumber", blockTag, false)
	if blockResults.err != nil {
		return blockAuditResult{blockNumber: blockNumber, err: blockResults.err}
	}
	checks += blockResults.checks
	findings = append(findings, blockResults.findings...)

	baselineBlock, ok := blockResults.results[endpointKey(a.baseline)]
	if !ok {
		return blockAuditResult{
			blockNumber: blockNumber,
			checks:      checks,
			findings:    findings,
		}
	}

	txHashes := make(map[string]struct{})
	addresses := make(map[string]struct{})
	collectBlockReferences(baselineBlock, txHashes, addresses)

	for _, txHash := range sortedKeys(txHashes) {
		transactionResults := a.compareCall(ctx, "eth_getTransactionByHash", txHash)
		if transactionResults.err != nil {
			return blockAuditResult{blockNumber: blockNumber, err: transactionResults.err}
		}
		checks += transactionResults.checks
		findings = append(findings, transactionResults.findings...)
		if baselineTransaction, ok := transactionResults.results[endpointKey(a.baseline)]; ok {
			collectTransactionAddresses(baselineTransaction, addresses)
		}

		receiptResults := a.compareCall(ctx, "eth_getTransactionReceipt", txHash)
		if receiptResults.err != nil {
			return blockAuditResult{blockNumber: blockNumber, err: receiptResults.err}
		}
		checks += receiptResults.checks
		findings = append(findings, receiptResults.findings...)
	}

	stateTags := []string{blockTag}
	if previous, ok := rpc.PreviousBlockTag(blockNumber); ok {
		stateTags = append(stateTags, previous)
	}

	for _, address := range sortedKeys(addresses) {
		for _, tag := range stateTags {
			balanceResults := a.compareCall(ctx, "eth_getBalance", address, tag)
			if balanceResults.err != nil {
				return blockAuditResult{blockNumber: blockNumber, err: balanceResults.err}
			}
			checks += balanceResults.checks
			findings = append(findings, balanceResults.findings...)

			nonceResults := a.compareCall(ctx, "eth_getTransactionCount", address, tag)
			if nonceResults.err != nil {
				return blockAuditResult{blockNumber: blockNumber, err: nonceResults.err}
			}
			checks += nonceResults.checks
			findings = append(findings, nonceResults.findings...)
		}
	}

	return blockAuditResult{
		blockNumber: blockNumber,
		checks:      checks,
		findings:    findings,
	}
}

func (a *Auditor) compareCall(ctx context.Context, method string, params ...any) compareCallResult {
	if err := ctx.Err(); err != nil {
		return compareCallResult{err: err}
	}

	endpoints := make([]Endpoint, 0, len(a.peers)+1)
	endpoints = append(endpoints, a.baseline)
	endpoints = append(endpoints, a.peers...)

	responses := make(chan endpointResponse, len(endpoints))
	for _, endpoint := range endpoints {
		go func(endpoint Endpoint) {
			response, _, err := endpoint.Provider.Call(ctx, method, params...)
			responses <- endpointResponse{
				endpoint: endpoint,
				response: response,
				err:      err,
			}
		}(endpoint)
	}

	byEndpoint := make(map[string]endpointResponse, len(endpoints))
	for index := 0; index < len(endpoints); index++ {
		response := <-responses
		byEndpoint[endpointKey(response.endpoint)] = response
	}

	if err := ctx.Err(); err != nil {
		return compareCallResult{err: err}
	}

	result := compareCallResult{
		checks:  len(a.peers),
		results: make(map[string]json.RawMessage, len(endpoints)),
	}

	baselineResponse := byEndpoint[endpointKey(a.baseline)]
	if baselineResponse.err == nil && baselineResponse.response != nil {
		result.results[endpointKey(a.baseline)] = baselineResponse.response.Result
	}

	for _, peer := range a.peers {
		peerResponse := byEndpoint[endpointKey(peer)]
		if peerResponse.err == nil && peerResponse.response != nil {
			result.results[endpointKey(peer)] = peerResponse.response.Result
		}

		switch {
		case baselineResponse.err != nil && peerResponse.err != nil:
			if baselineResponse.err.Error() != peerResponse.err.Error() {
				result.findings = append(result.findings, Finding{
					Endpoint: peer.Target.Name,
					Method:   method,
					Params:   append([]any(nil), params...),
					Error:    fmt.Sprintf("baseline error %q != peer error %q", baselineResponse.err, peerResponse.err),
				})
			}
		case baselineResponse.err != nil || peerResponse.err != nil:
			result.findings = append(result.findings, Finding{
				Endpoint: peer.Target.Name,
				Method:   method,
				Params:   append([]any(nil), params...),
				Error:    fmt.Sprintf("baseline error %v / peer error %v", baselineResponse.err, peerResponse.err),
			})
		default:
			differences, err := CompareJSON(baselineResponse.response.Result, peerResponse.response.Result, a.options)
			if err != nil {
				result.findings = append(result.findings, Finding{
					Endpoint: peer.Target.Name,
					Method:   method,
					Params:   append([]any(nil), params...),
					Error:    err.Error(),
				})
				continue
			}
			if len(differences) > 0 {
				result.findings = append(result.findings, Finding{
					Endpoint:    peer.Target.Name,
					Method:      method,
					Params:      append([]any(nil), params...),
					Differences: differences,
				})
			}
		}
	}

	return result
}

type endpointResponse struct {
	endpoint Endpoint
	response *rpc.Response
	err      error
}

func endpointKey(endpoint Endpoint) string {
	if endpoint.Target.URL != "" {
		return endpoint.Target.URL
	}
	return endpoint.Target.Name
}

func collectBlockReferences(raw json.RawMessage, txHashes, addresses map[string]struct{}) {
	if isNullJSON(raw) {
		return
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}

	addAddress(addresses, stringField(payload["miner"]))

	transactions, ok := payload["transactions"].([]any)
	if !ok {
		return
	}

	for _, item := range transactions {
		switch typed := item.(type) {
		case string:
			addHash(txHashes, typed)
		case map[string]any:
			addHash(txHashes, stringField(typed["hash"]))
			addAddress(addresses, stringField(typed["from"]))
			addAddress(addresses, stringField(typed["to"]))
		}
	}
}

func collectTransactionAddresses(raw json.RawMessage, addresses map[string]struct{}) {
	if isNullJSON(raw) {
		return
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return
	}

	addAddress(addresses, stringField(payload["from"]))
	addAddress(addresses, stringField(payload["to"]))
}

func isNullJSON(raw json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(raw))
	return trimmed == "" || trimmed == "null"
}

func addHash(hashes map[string]struct{}, value string) {
	key := strings.ToLower(strings.TrimSpace(value))
	if key == "" {
		return
	}
	hashes[key] = struct{}{}
}

func addAddress(addresses map[string]struct{}, value string) {
	key := strings.ToLower(strings.TrimSpace(value))
	if key == "" || key == "0x0000000000000000000000000000000000000000" {
		return
	}
	addresses[key] = struct{}{}
}

func stringField(value any) string {
	typed, _ := value.(string)
	return typed
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortFindings(findings []Finding) {
	sort.Slice(findings, func(i, j int) bool {
		return findingSortKey(findings[i]) < findingSortKey(findings[j])
	})
}

func findingSortKey(finding Finding) string {
	params, err := json.Marshal(finding.Params)
	if err != nil {
		return finding.Endpoint + "|" + finding.Method + "|" + fmt.Sprint(finding.Params)
	}
	return finding.Endpoint + "|" + finding.Method + "|" + string(params)
}

func defaultAuditorConcurrency() int {
	concurrency := runtime.GOMAXPROCS(0)
	if concurrency < 1 {
		return 1
	}
	if concurrency > 8 {
		return 8
	}
	return concurrency
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
