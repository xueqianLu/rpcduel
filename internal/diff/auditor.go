package diff

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

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

type Auditor struct {
	baseline Endpoint
	peers    []Endpoint
	options  Options
}

func NewAuditor(baseline Endpoint, peers []Endpoint, options Options) *Auditor {
	return &Auditor{
		baseline: baseline,
		peers:    peers,
		options:  options,
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

	for blockNumber := from; blockNumber <= to; blockNumber++ {
		blockTag := rpc.HexBlockNumber(blockNumber)

		blockResults := a.compareCall(ctx, report, "eth_getBlockByNumber", blockTag, false)
		baselineBlock, ok := blockResults[a.baseline.Target.Name]
		if !ok {
			continue
		}

		txHashes := make(map[string]struct{})
		addresses := make(map[string]struct{})
		collectBlockReferences(baselineBlock, txHashes, addresses)

		for _, txHash := range sortedKeys(txHashes) {
			transactionResults := a.compareCall(ctx, report, "eth_getTransactionByHash", txHash)
			if baselineTransaction, ok := transactionResults[a.baseline.Target.Name]; ok {
				collectTransactionAddresses(baselineTransaction, addresses)
			}
			a.compareCall(ctx, report, "eth_getTransactionReceipt", txHash)
		}

		stateTags := []string{blockTag}
		if previous, ok := rpc.PreviousBlockTag(blockNumber); ok {
			stateTags = append(stateTags, previous)
		}

		for _, address := range sortedKeys(addresses) {
			for _, tag := range stateTags {
				a.compareCall(ctx, report, "eth_getBalance", address, tag)
				a.compareCall(ctx, report, "eth_getTransactionCount", address, tag)
			}
		}
	}

	return report, nil
}

func (a *Auditor) compareCall(ctx context.Context, report *Report, method string, params ...any) map[string]json.RawMessage {
	report.Checks += len(a.peers)

	results := make(map[string]json.RawMessage, len(a.peers)+1)
	baselineResponse, _, baselineErr := a.baseline.Provider.Call(ctx, method, params...)
	if baselineErr == nil && baselineResponse != nil {
		results[a.baseline.Target.Name] = baselineResponse.Result
	}

	for _, peer := range a.peers {
		peerResponse, _, peerErr := peer.Provider.Call(ctx, method, params...)
		if peerErr == nil && peerResponse != nil {
			results[peer.Target.Name] = peerResponse.Result
		}

		switch {
		case baselineErr != nil && peerErr != nil:
			if baselineErr.Error() != peerErr.Error() {
				report.Findings = append(report.Findings, Finding{
					Endpoint: peer.Target.Name,
					Method:   method,
					Params:   append([]any(nil), params...),
					Error:    fmt.Sprintf("baseline error %q != peer error %q", baselineErr, peerErr),
				})
			}
		case baselineErr != nil || peerErr != nil:
			report.Findings = append(report.Findings, Finding{
				Endpoint: peer.Target.Name,
				Method:   method,
				Params:   append([]any(nil), params...),
				Error:    fmt.Sprintf("baseline error %v / peer error %v", baselineErr, peerErr),
			})
		default:
			differences, err := CompareJSON(baselineResponse.Result, peerResponse.Result, a.options)
			if err != nil {
				report.Findings = append(report.Findings, Finding{
					Endpoint: peer.Target.Name,
					Method:   method,
					Params:   append([]any(nil), params...),
					Error:    err.Error(),
				})
				continue
			}
			if len(differences) > 0 {
				report.Findings = append(report.Findings, Finding{
					Endpoint:    peer.Target.Name,
					Method:      method,
					Params:      append([]any(nil), params...),
					Differences: differences,
				})
			}
		}
	}

	return results
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
