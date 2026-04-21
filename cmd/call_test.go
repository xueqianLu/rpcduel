// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	os.Stdout = w
	defer func() {
		os.Stdout = old
	}()

	fn()

	_ = w.Close()
	data, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	return string(data)
}

func resetCallGlobals() {
	callRPC = ""
	callMethod = "eth_blockNumber"
	callParamsStr = "[]"
	callParamsFile = ""
	callTimeout = 30 * time.Second
	callOutput = "text"
	for _, name := range []string{"method", "params", "params-file", "rpc", "output", "timeout"} {
		if flag := callCmd.Flags().Lookup(name); flag != nil {
			flag.Changed = false
		}
	}
}

func TestRunCall_JSONSuccess(t *testing.T) {
	resetCallGlobals()
	defer resetCallGlobals()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  map[string]interface{}{"number": "0x1"},
		})
	}))
	defer srv.Close()

	callRPC = srv.URL
	callMethod = "eth_getBlockByNumber"
	callParamsStr = `["latest", false]`
	callOutput = "json"

	out := captureStdout(t, func() {
		if err := runCall(callCmd, nil); err != nil {
			t.Fatalf("runCall: %v", err)
		}
	})

	var got map[string]interface{}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal output: %v\noutput=%s", err, out)
	}
	if got["method"] != "eth_getBlockByNumber" {
		t.Fatalf("expected method in output, got %v", got["method"])
	}
	if got["success"] != true {
		t.Fatalf("expected success=true, got %v", got["success"])
	}
	result, ok := got["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected object result, got %#v", got["result"])
	}
	if result["number"] != "0x1" {
		t.Fatalf("expected block number 0x1, got %v", result["number"])
	}
}

func TestRunCall_RPCError(t *testing.T) {
	resetCallGlobals()
	defer resetCallGlobals()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"error": map[string]interface{}{
				"code":    -32000,
				"message": "execution reverted",
			},
		})
	}))
	defer srv.Close()

	callRPC = srv.URL
	callMethod = "debug_traceTransaction"
	callParamsStr = `["0xdeadbeef"]`
	callOutput = "text"

	out := captureStdout(t, func() {
		err := runCall(callCmd, nil)
		if err == nil {
			t.Fatal("expected error")
		}
	})

	for _, want := range []string{"Error:", "execution reverted", "code=-32000"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected output to contain %q, got: %s", want, out)
		}
	}
}

func TestLoadCallParams_FromFile(t *testing.T) {
	resetCallGlobals()
	defer resetCallGlobals()

	dir := t.TempDir()
	path := filepath.Join(dir, "params.json")
	if err := os.WriteFile(path, []byte(`["latest", false]`), 0o644); err != nil {
		t.Fatalf("write params file: %v", err)
	}

	callParamsFile = path
	callParamsStr = `["ignored"]`
	if flag := callCmd.Flags().Lookup("params-file"); flag != nil {
		flag.Changed = true
	}

	params, err := loadCallParams(callCmd, nil)
	if err != nil {
		t.Fatalf("loadCallParams: %v", err)
	}
	if len(params) != 2 {
		t.Fatalf("expected 2 params, got %d", len(params))
	}
	if params[0] != "latest" {
		t.Fatalf("expected first param latest, got %v", params[0])
	}
}

func TestRunCall_PositionalArgs(t *testing.T) {
	resetCallGlobals()
	defer resetCallGlobals()

	var gotMethod string
	var gotParams []interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		gotMethod, _ = req["method"].(string)
		gotParams, _ = req["params"].([]interface{})
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req["id"],
			"result":  "0x123",
		})
	}))
	defer srv.Close()

	callRPC = srv.URL
	out := captureStdout(t, func() {
		if err := runCall(callCmd, []string{"eth_getBalance", "0xa11111", "latest"}); err != nil {
			t.Fatalf("runCall: %v", err)
		}
	})

	if gotMethod != "eth_getBalance" {
		t.Fatalf("expected method eth_getBalance, got %q", gotMethod)
	}
	wantParams := []interface{}{"0xa11111", "latest"}
	if !reflect.DeepEqual(gotParams, wantParams) {
		t.Fatalf("expected params %v, got %v", wantParams, gotParams)
	}
	if !strings.Contains(out, `"0x123"`) {
		t.Fatalf("expected result in output, got: %s", out)
	}
}

func TestLoadCallParams_PositionalConflictWithFlag(t *testing.T) {
	resetCallGlobals()
	defer resetCallGlobals()

	callParamsStr = `["latest"]`
	if flag := callCmd.Flags().Lookup("params"); flag != nil {
		flag.Changed = true
	}

	_, err := loadCallParams(callCmd, []string{"eth_getBalance", "0xa11111", "latest"})
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !strings.Contains(err.Error(), "cannot use positional params and --params together") {
		t.Fatalf("unexpected error: %v", err)
	}
}
