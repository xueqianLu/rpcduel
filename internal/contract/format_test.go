// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package contract

import (
	"math/big"
	"testing"
)

func TestFormatUnits(t *testing.T) {
	cases := []struct {
		raw      string
		decimals uint8
		want     string
	}{
		{"0", 18, "0"},
		{"1", 18, "0.000000000000000001"},
		{"1500000000000000000", 18, "1.5"},
		{"1234567890", 6, "1234.56789"},
		{"1234567890", 0, "1234567890"},
		{"1000000", 6, "1"},
		{"-1500", 2, "-15"},
	}
	for _, tc := range cases {
		raw, _ := new(big.Int).SetString(tc.raw, 10)
		got := FormatUnits(raw, tc.decimals)
		if got != tc.want {
			t.Errorf("FormatUnits(%s, %d) = %q, want %q", tc.raw, tc.decimals, got, tc.want)
		}
	}
}

func TestFormatBalanceLine(t *testing.T) {
	raw, _ := new(big.Int).SetString("1234567890", 10)
	got := FormatBalanceLine(raw, 6, "USDC")
	want := "1234.56789 USDC (raw: 1234567890)"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
