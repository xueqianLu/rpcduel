// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

// Package doctor probes JSON-RPC endpoints for connectivity, identity,
// sync state, and method capability. The Run helper returns a
// structured Report that can be rendered as either a human-readable
// table or JSON.
package doctor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/xueqianLu/rpcduel/internal/rpc"
)

// Probe is the result of a single JSON-RPC probe against one endpoint.
type Probe struct {
	Method  string        `json:"method"`
	OK      bool          `json:"ok"`
	Latency time.Duration `json:"latency_ns"`
	Value   string        `json:"value,omitempty"`
	Error   string        `json:"error,omitempty"`
}

// EndpointReport captures every probe done against a single endpoint
// plus a few derived facts (chain id, block height, sync state).
type EndpointReport struct {
	URL          string  `json:"url"`
	Reachable    bool    `json:"reachable"`
	ChainID      string  `json:"chain_id,omitempty"`
	ClientVer    string  `json:"client_version,omitempty"`
	BlockNumber  uint64  `json:"block_number,omitempty"`
	Syncing      bool    `json:"syncing"`
	PeerCount    uint64  `json:"peer_count,omitempty"`
	GasPriceWei  string  `json:"gas_price_wei,omitempty"`
	Probes       []Probe `json:"probes"`
	FailedProbes int     `json:"failed_probes"`
}

// Report is the aggregate result of a doctor run across N endpoints.
type Report struct {
	GeneratedAt time.Time        `json:"generated_at"`
	Endpoints   []EndpointReport `json:"endpoints"`
}

// Options configures a doctor run.
type Options struct {
	Timeout      time.Duration
	ExtraMethods []string
}

// Run probes every endpoint in urls in parallel and returns the
// aggregated report. Per-endpoint failures are recorded inline so the
// report is always renderable even when every endpoint is down.
func Run(ctx context.Context, mkClient func(string) *rpc.Client, urls []string, opts Options) *Report {
	report := &Report{GeneratedAt: time.Now().UTC(), Endpoints: make([]EndpointReport, len(urls))}
	var wg sync.WaitGroup
	for i, u := range urls {
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			report.Endpoints[i] = probeOne(ctx, mkClient(u), u, opts)
		}(i, u)
	}
	wg.Wait()
	return report
}

func probeOne(ctx context.Context, c *rpc.Client, url string, opts Options) EndpointReport {
	er := EndpointReport{URL: url}
	defer c.Close()

	core := []struct {
		name   string
		params []interface{}
	}{
		{"web3_clientVersion", nil},
		{"eth_chainId", nil},
		{"eth_blockNumber", nil},
		{"eth_syncing", nil},
		{"net_peerCount", nil},
		{"eth_gasPrice", nil},
	}
	for _, p := range core {
		probe := callProbe(ctx, c, p.name, p.params)
		er.Probes = append(er.Probes, probe)
		if !probe.OK {
			er.FailedProbes++
			continue
		}
		switch p.name {
		case "web3_clientVersion":
			er.ClientVer = unquote(probe.Value)
			er.Reachable = true
		case "eth_chainId":
			er.ChainID = probe.Value
			er.Reachable = true
		case "eth_blockNumber":
			er.BlockNumber = parseHexUint(probe.Value)
			er.Reachable = true
		case "eth_syncing":
			er.Syncing = probe.Value != "false" && probe.Value != "null"
		case "net_peerCount":
			er.PeerCount = parseHexUint(probe.Value)
		case "eth_gasPrice":
			er.GasPriceWei = probe.Value
		}
	}

	for _, m := range opts.ExtraMethods {
		probe := callProbe(ctx, c, m, nil)
		er.Probes = append(er.Probes, probe)
		if !probe.OK {
			er.FailedProbes++
		}
	}
	return er
}

func callProbe(ctx context.Context, c *rpc.Client, method string, params []interface{}) Probe {
	resp, dur, err := c.Call(ctx, method, params)
	p := Probe{Method: method, Latency: dur}
	if err != nil {
		p.Error = err.Error()
		return p
	}
	if resp.Error != nil {
		p.Error = resp.Error.Error()
		return p
	}
	p.OK = true
	if len(resp.Result) > 0 {
		p.Value = strings.TrimSpace(string(resp.Result))
	}
	return p
}

func unquote(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		out, err := strconv.Unquote(s)
		if err == nil {
			return out
		}
	}
	return s
}

func parseHexUint(s string) uint64 {
	s = unquote(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if s == "" {
		return 0
	}
	v, err := strconv.ParseUint(s, 16, 64)
	if err != nil {
		return 0
	}
	return v
}

// PrintText renders the report as a human-readable table.
func PrintText(w io.Writer, r *Report) {
	for i, ep := range r.Endpoints {
		if i > 0 {
			fmt.Fprintln(w)
		}
		status := "UP"
		if !ep.Reachable {
			status = "DOWN"
		}
		fmt.Fprintf(w, "Endpoint: %s [%s]\n", ep.URL, status)
		if ep.ClientVer != "" {
			fmt.Fprintf(w, "  Client:        %s\n", ep.ClientVer)
		}
		if ep.ChainID != "" {
			fmt.Fprintf(w, "  Chain ID:      %s (%d)\n", unquote(ep.ChainID), parseHexUint(ep.ChainID))
		}
		if ep.Reachable {
			fmt.Fprintf(w, "  Block:         %d\n", ep.BlockNumber)
			fmt.Fprintf(w, "  Syncing:       %t\n", ep.Syncing)
			fmt.Fprintf(w, "  Peers:         %d\n", ep.PeerCount)
			if ep.GasPriceWei != "" {
				fmt.Fprintf(w, "  Gas price:     %s wei\n", unquote(ep.GasPriceWei))
			}
		}
		fmt.Fprintf(w, "  Probes:        %d / %d ok\n", len(ep.Probes)-ep.FailedProbes, len(ep.Probes))
		for _, p := range ep.Probes {
			tag := "ok"
			if !p.OK {
				tag = "ERR"
			}
			line := fmt.Sprintf("    [%s] %-26s  %6s", tag, p.Method, durMs(p.Latency))
			if p.Error != "" {
				line += "  " + truncate(p.Error, 80)
			}
			fmt.Fprintln(w, line)
		}
	}
}

// PrintJSON renders the report as machine-readable JSON.
func PrintJSON(w io.Writer, r *Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// HasFailures returns true if any endpoint had at least one failed
// probe or was completely unreachable.
func (r *Report) HasFailures() bool {
	if r == nil {
		return false
	}
	for _, ep := range r.Endpoints {
		if !ep.Reachable || ep.FailedProbes > 0 {
			return true
		}
	}
	return false
}

func durMs(d time.Duration) string {
	if d == 0 {
		return "-"
	}
	return fmt.Sprintf("%dms", d.Milliseconds())
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

// ErrNoEndpoints is returned by callers when neither --rpc nor config
// supplied any endpoint URLs.
var ErrNoEndpoints = errors.New("at least one --rpc endpoint is required")
