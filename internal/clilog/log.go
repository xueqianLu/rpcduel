// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

// Package clilog configures the process-wide slog logger for rpcduel commands.
package clilog

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Setup configures the default slog logger to write to stderr using the
// requested level and format. Format must be "text" or "json"; level must be
// one of debug/info/warn/error.
func Setup(level, format string) error {
	lvl, err := parseLevel(level)
	if err != nil {
		return err
	}
	handler, err := buildHandler(os.Stderr, lvl, format)
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(handler))
	return nil
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid log level %q (want debug|info|warn|error)", s)
	}
}

func buildHandler(w io.Writer, lvl slog.Level, format string) (slog.Handler, error) {
	opts := &slog.HandlerOptions{Level: lvl}
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "text":
		return slog.NewTextHandler(w, opts), nil
	case "json":
		return slog.NewJSONHandler(w, opts), nil
	default:
		return nil, fmt.Errorf("invalid log format %q (want text|json)", format)
	}
}
