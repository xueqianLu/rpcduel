// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"strings"
	"testing"
)

func TestParseReplayOnlyTargets_Aliases(t *testing.T) {
	got, err := parseReplayOnlyTargets([]string{"account", "trace"})
	if err != nil {
		t.Fatalf("parseReplayOnlyTargets: %v", err)
	}
	for _, want := range []string{"balance", "transaction_count", "trace_transaction", "trace_block"} {
		if !got[want] {
			t.Fatalf("expected target %q in parsed set, got %v", want, got)
		}
	}
}
func TestParseBenchgenOnlyTargets_Aliases(t *testing.T) {
	got, err := parseBenchgenOnlyTargets([]string{"transaction", "logs", "trace_block"})
	if err != nil {
		t.Fatalf("parseBenchgenOnlyTargets: %v", err)
	}
	for _, want := range []string{"transaction_by_hash", "transaction_receipt", "get_logs", "debug_trace_block"} {
		if !got[want] {
			t.Fatalf("expected target %q in parsed set, got %v", want, got)
		}
	}
}
func TestParseOnlyTargets_Invalid(t *testing.T) {
	_, err := parseReplayOnlyTargets([]string{"nope"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unknown target") {
		t.Fatalf("unexpected error: %v", err)
	}
}
func TestRunBenchgen_OnlyConflictsWithTraceFlags(t *testing.T) {
	resetBenchgenGlobals()
	defer resetBenchgenGlobals()
	benchgenOnly = []string{"balance"}
	benchgenTraceTx = true
	benchgenOut = "dummy.json"
	if err := runBenchgen(nil, nil); err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestRunReplay_OnlyConflictsWithTraceFlags(t *testing.T) {
	diffTestRPCs = []string{"http://a", "http://b"}
	diffTestOnly = []string{"balance"}
	diffTestTraceTransaction = true
	defer func() {
		diffTestRPCs = nil
		diffTestOnly = nil
		diffTestTraceTransaction = false
	}()
	if err := runDiffTest(nil, nil); err == nil {
		t.Fatal("expected conflict error")
	}
}
