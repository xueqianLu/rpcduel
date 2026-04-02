package cmd

import (
	"fmt"
	"sort"
	"strings"
)

func parseReplayOnlyTargets(raw []string) (map[string]bool, error) {
	aliases := map[string][]string{
		"account":     {"balance", "transaction_count"},
		"accounts":    {"balance", "transaction_count"},
		"transaction": {"transaction_by_hash", "transaction_receipt"},
		"tx":          {"transaction_by_hash", "transaction_receipt"},
		"block":       {"block_by_number"},
		"trace":       {"trace_transaction", "trace_block"},
	}
	allowed := map[string]bool{
		"balance":             true,
		"transaction_count":   true,
		"transaction_by_hash": true,
		"transaction_receipt": true,
		"block_by_number":     true,
		"trace_transaction":   true,
		"trace_block":         true,
	}
	return parseOnlyTargets(raw, aliases, allowed)
}

func parseBenchgenOnlyTargets(raw []string) (map[string]bool, error) {
	aliases := map[string][]string{
		"account":           {"balance", "transaction_count", "mixed_balance"},
		"accounts":          {"balance", "transaction_count", "mixed_balance"},
		"transaction":       {"transaction_by_hash", "transaction_receipt"},
		"tx":                {"transaction_by_hash", "transaction_receipt"},
		"block":             {"block_by_number", "get_logs"},
		"logs":              {"get_logs"},
		"trace":             {"debug_trace_transaction", "debug_trace_block"},
		"trace_transaction": {"debug_trace_transaction"},
		"trace_block":       {"debug_trace_block"},
	}
	allowed := map[string]bool{
		"balance":                 true,
		"transaction_count":       true,
		"transaction_by_hash":     true,
		"transaction_receipt":     true,
		"block_by_number":         true,
		"get_logs":                true,
		"mixed_balance":           true,
		"debug_trace_transaction": true,
		"debug_trace_block":       true,
	}
	return parseOnlyTargets(raw, aliases, allowed)
}

func parseOnlyTargets(raw []string, aliases map[string][]string, allowed map[string]bool) (map[string]bool, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	out := make(map[string]bool)
	for _, item := range raw {
		for _, token := range strings.Split(item, ",") {
			token = strings.TrimSpace(strings.ToLower(token))
			token = strings.ReplaceAll(token, "-", "_")
			if token == "" {
				continue
			}
			if expanded, ok := aliases[token]; ok {
				for _, name := range expanded {
					out[name] = true
				}
				continue
			}
			if !allowed[token] {
				return nil, fmt.Errorf("unknown target %q (allowed: %s)", token, strings.Join(sortedKeys(allowed), ", "))
			}
			out[token] = true
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
