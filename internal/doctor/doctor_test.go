// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package doctor

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/xueqianLu/rpcduel/internal/rpc"
)

func newServer(t *testing.T, replies map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
			ID     int    `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		id := strconv.Itoa(req.ID)
		w.Header().Set("Content-Type", "application/json")
		v, ok := replies[req.Method]
		if !ok {
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":` + id + `,"error":{"code":-32601,"message":"method not found"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":` + id + `,"result":` + v + `}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestRun_HappyPath(t *testing.T) {
	srv := newServer(t, map[string]string{
		"web3_clientVersion": `"Geth/v1.13.0"`,
		"eth_chainId":        `"0x1"`,
		"eth_blockNumber":    `"0x10"`,
		"eth_syncing":        `false`,
		"net_peerCount":      `"0x5"`,
		"eth_gasPrice":       `"0x3b9aca00"`,
	})
	mk := func(u string) *rpc.Client { return rpc.NewClient(u, 2*time.Second) }
	rep := Run(context.Background(), mk, []string{srv.URL}, Options{Timeout: 2 * time.Second})
	if len(rep.Endpoints) != 1 {
		t.Fatalf("want 1 endpoint, got %d", len(rep.Endpoints))
	}
	ep := rep.Endpoints[0]
	if !ep.Reachable {
		t.Fatalf("expected reachable, got %+v", ep)
	}
	if ep.BlockNumber != 16 {
		t.Errorf("block=%d, want 16", ep.BlockNumber)
	}
	if ep.PeerCount != 5 {
		t.Errorf("peers=%d, want 5", ep.PeerCount)
	}
	if ep.Syncing {
		t.Errorf("syncing=true, want false")
	}
	if ep.FailedProbes != 0 {
		t.Errorf("failed=%d, want 0", ep.FailedProbes)
	}
	if rep.HasFailures() {
		t.Errorf("HasFailures=true, want false")
	}
	var b bytes.Buffer
	PrintText(&b, rep)
	if !strings.Contains(b.String(), "Geth/v1.13.0") {
		t.Errorf("text output missing client version: %s", b.String())
	}
}

func TestRun_ExtraMethodFails(t *testing.T) {
	srv := newServer(t, map[string]string{
		"web3_clientVersion": `"X"`,
		"eth_chainId":        `"0x1"`,
		"eth_blockNumber":    `"0x1"`,
		"eth_syncing":        `false`,
		"net_peerCount":      `"0x0"`,
		"eth_gasPrice":       `"0x0"`,
	})
	mk := func(u string) *rpc.Client { return rpc.NewClient(u, 2*time.Second) }
	rep := Run(context.Background(), mk, []string{srv.URL}, Options{
		Timeout:      2 * time.Second,
		ExtraMethods: []string{"debug_traceBlock"},
	})
	if rep.Endpoints[0].FailedProbes != 1 {
		t.Errorf("want 1 failed probe, got %d", rep.Endpoints[0].FailedProbes)
	}
	if !rep.HasFailures() {
		t.Errorf("HasFailures should be true")
	}
}
