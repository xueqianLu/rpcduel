// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package diff_test

import (
	"encoding/json"
	"testing"

	"github.com/xueqianLu/rpcduel/internal/diff"
)

func TestCompare_Equal(t *testing.T) {
	opts := diff.DefaultOptions()
	diffs, err := diff.Compare(json.RawMessage(`"0x1a"`), json.RawMessage(`"26"`), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 0 {
		t.Errorf("expected 0 diffs (hex==decimal), got %d: %v", len(diffs), diffs)
	}
}

func TestCompare_HexDecimalEqual(t *testing.T) {
	opts := diff.DefaultOptions()
	diffs, err := diff.Compare(json.RawMessage(`{"number":"0x10"}`), json.RawMessage(`{"number":"16"}`), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 0 {
		t.Errorf("expected no diffs, got %v", diffs)
	}
}

func TestCompare_ValueDiff(t *testing.T) {
	opts := diff.DefaultOptions()
	diffs, err := diff.Compare(json.RawMessage(`{"a":1}`), json.RawMessage(`{"a":2}`), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) == 0 {
		t.Error("expected at least 1 diff")
	}
}

func TestCompare_MissingField(t *testing.T) {
	opts := diff.DefaultOptions()
	diffs, err := diff.Compare(json.RawMessage(`{"a":1,"b":2}`), json.RawMessage(`{"a":1}`), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) == 0 {
		t.Error("expected diff for missing field b")
	}
	if diffs[0].Reason != "missing in right" {
		t.Errorf("unexpected reason: %s", diffs[0].Reason)
	}
}

func TestCompare_IgnoreField(t *testing.T) {
	opts := diff.DefaultOptions()
	opts.IgnoreFields["timestamp"] = true
	diffs, err := diff.Compare(
		json.RawMessage(`{"a":1,"timestamp":"100"}`),
		json.RawMessage(`{"a":1,"timestamp":"999"}`),
		opts,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 0 {
		t.Errorf("expected 0 diffs after ignoring timestamp, got %v", diffs)
	}
}

func TestCompare_ArrayOrderSensitive(t *testing.T) {
	opts := diff.DefaultOptions()
	opts.IgnoreOrder = false
	diffs, err := diff.Compare(json.RawMessage(`[1,2,3]`), json.RawMessage(`[3,2,1]`), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) == 0 {
		t.Error("expected diffs for different order")
	}
}

func TestCompare_ArrayOrderInsensitive(t *testing.T) {
	opts := diff.DefaultOptions()
	opts.IgnoreOrder = true
	diffs, err := diff.Compare(json.RawMessage(`[1,2,3]`), json.RawMessage(`[3,2,1]`), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 0 {
		t.Errorf("expected 0 diffs with ignore-order, got %v", diffs)
	}
}

func TestCompare_NestedObjects(t *testing.T) {
	opts := diff.DefaultOptions()
	left := `{"block":{"number":"0xa","hash":"0xabc"}}`
	right := `{"block":{"number":"0xa","hash":"0xabc"}}`
	diffs, err := diff.Compare(json.RawMessage(left), json.RawMessage(right), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 0 {
		t.Errorf("expected no diffs, got %v", diffs)
	}
}

func TestCompare_NestedObjectsDifferent(t *testing.T) {
	opts := diff.DefaultOptions()
	left := `{"block":{"number":"0xa","hash":"0xabc"}}`
	right := `{"block":{"number":"0xa","hash":"0xdef"}}`
	diffs, err := diff.Compare(json.RawMessage(left), json.RawMessage(right), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) == 0 {
		t.Error("expected diff for different hash")
	}
}

func TestCompare_NullValues(t *testing.T) {
	opts := diff.DefaultOptions()
	diffs, err := diff.Compare(json.RawMessage(`null`), json.RawMessage(`null`), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 0 {
		t.Errorf("expected no diffs for null==null, got %v", diffs)
	}
}

func TestCompare_ArrayLengthMismatch(t *testing.T) {
	opts := diff.DefaultOptions()
	diffs, err := diff.Compare(json.RawMessage(`[1,2,3]`), json.RawMessage(`[1,2]`), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) == 0 {
		t.Error("expected diff for array length mismatch")
	}
}
