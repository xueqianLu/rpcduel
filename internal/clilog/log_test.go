// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package clilog_test

import (
	"log/slog"
	"testing"

	"github.com/xueqianLu/rpcduel/internal/clilog"
)

func TestSetup(t *testing.T) {
	cases := []struct {
		level, format string
		wantErr       bool
	}{
		{"info", "text", false},
		{"debug", "json", false},
		{"warn", "text", false},
		{"warning", "json", false},
		{"error", "json", false},
		{"", "", false},
		{"trace", "text", true},
		{"info", "yaml", true},
	}
	for _, tc := range cases {
		err := clilog.Setup(tc.level, tc.format)
		if (err != nil) != tc.wantErr {
			t.Errorf("Setup(%q,%q) err=%v wantErr=%v", tc.level, tc.format, err, tc.wantErr)
		}
	}
	slog.Info("smoke", "after", "setup")
}
