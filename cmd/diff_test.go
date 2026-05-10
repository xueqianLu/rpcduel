// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package cmd

import "testing"

func TestParseRequestsFile_JSONArray(t *testing.T) {
	in := []byte(`[{"method":"eth_blockNumber","params":[]},{"method":"eth_chainId","params":[]}]`)
	reqs, err := parseRequestsFile(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(reqs) != 2 || reqs[0].Method != "eth_blockNumber" || reqs[1].Method != "eth_chainId" {
		t.Fatalf("unexpected reqs: %+v", reqs)
	}
}

func TestParseRequestsFile_SingleObject(t *testing.T) {
	in := []byte(`{"jsonrpc":"2.0","id":1,"method":"eth_call","params":[{"to":"0x0"},"latest"]}`)
	reqs, err := parseRequestsFile(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(reqs) != 1 || reqs[0].Method != "eth_call" || len(reqs[0].Params) != 2 {
		t.Fatalf("unexpected reqs: %+v", reqs)
	}
}

func TestParseRequestsFile_NDJSON(t *testing.T) {
	in := []byte(`# captured from access log
{"jsonrpc":"2.0","id":2076,"method":"eth_call","params":[{"data":"0x06fdde03","to":"0x0000000000000000000000000000000000000100"},"0xa0"]}

{"jsonrpc":"2.0","id":2077,"method":"eth_call","params":[{"data":"0x95d89b41","to":"0x0000000000000000000000000000000000000100"},"latest"]}
`)
	reqs, err := parseRequestsFile(in)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(reqs) != 2 {
		t.Fatalf("want 2 reqs, got %d", len(reqs))
	}
	if reqs[0].Method != "eth_call" || reqs[1].Method != "eth_call" {
		t.Fatalf("unexpected methods: %+v", reqs)
	}
	if reqs[0].Params[1] != "0xa0" || reqs[1].Params[1] != "latest" {
		t.Fatalf("unexpected params: %+v", reqs)
	}
}

func TestParseRequestsFile_NDJSON_BadLine(t *testing.T) {
	in := []byte(`{"method":"eth_blockNumber","params":[]}
not-json
`)
	if _, err := parseRequestsFile(in); err == nil {
		t.Fatal("expected error for bad line")
	}
}

func TestParseRequestsFile_MissingMethod(t *testing.T) {
	in := []byte(`{"jsonrpc":"2.0","id":1,"params":[]}
{"method":"eth_blockNumber","params":[]}
`)
	if _, err := parseRequestsFile(in); err == nil {
		t.Fatal("expected error for missing method")
	}
}
