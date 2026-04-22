// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package contract

import (
	"fmt"
	"math/big"
	"strings"
)

// FormatUnits renders raw as a decimal number with `decimals` digits to
// the right of the point, trimming insignificant trailing zeros. For
// the common case decimals=18, FormatUnits(1_500_000_000_000_000_000) -> "1.5".
func FormatUnits(raw *big.Int, decimals uint8) string {
	if raw == nil {
		return ""
	}
	if decimals == 0 {
		return raw.String()
	}
	neg := raw.Sign() < 0
	abs := new(big.Int).Abs(raw)
	s := abs.String()

	// Left-pad so len(s) > decimals.
	for len(s) <= int(decimals) {
		s = "0" + s
	}
	pointAt := len(s) - int(decimals)
	intPart := s[:pointAt]
	fracPart := strings.TrimRight(s[pointAt:], "0")

	out := intPart
	if fracPart != "" {
		out = intPart + "." + fracPart
	}
	if neg {
		out = "-" + out
	}
	return out
}

// FormatBalanceLine returns "<formatted> <symbol> (raw: <raw>)" — the
// canonical one-line balance representation in rpcduel.
func FormatBalanceLine(raw *big.Int, decimals uint8, symbol string) string {
	if symbol == "" {
		symbol = "(unknown)"
	}
	return fmt.Sprintf("%s %s (raw: %s)", FormatUnits(raw, decimals), symbol, raw.String())
}
