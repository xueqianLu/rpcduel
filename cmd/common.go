package cmd

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/xueqianLu/rpcduel/pkg/rpc"
)

func resolver() (*rpc.Resolver, error) {
	return rpc.NewResolver(providerMappings)
}

func resolveTarget(raw string) (rpc.Target, error) {
	resolver, err := resolver()
	if err != nil {
		return rpc.Target{}, err
	}
	return resolver.Resolve(raw)
}

func resolveTargets(rawTargets []string) ([]rpc.Target, error) {
	resolver, err := resolver()
	if err != nil {
		return nil, err
	}

	targets := make([]rpc.Target, 0, len(rawTargets))
	for _, raw := range rawTargets {
		target, err := resolver.Resolve(raw)
		if err != nil {
			return nil, err
		}
		targets = append(targets, target)
	}
	return targets, nil
}

func newProvider(raw string, timeout time.Duration) (*rpc.Provider, error) {
	target, err := resolveTarget(raw)
	if err != nil {
		return nil, err
	}
	return rpc.NewProvider(target, timeout), nil
}

func parseCLIParams(args []string) ([]any, error) {
	params := make([]any, 0, len(args))
	for _, arg := range args {
		value, err := parseCLIParam(arg)
		if err != nil {
			return nil, err
		}
		params = append(params, value)
	}
	return params, nil
}

func parseCLIParam(raw string) (any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	if shouldDecodeCLIParam(trimmed) {
		var value any
		if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
			return nil, fmt.Errorf("parse CLI param %q: %w", raw, err)
		}
		return value, nil
	}

	return raw, nil
}

func shouldDecodeCLIParam(value string) bool {
	switch value {
	case "true", "false", "null":
		return true
	}

	if len(value) == 0 {
		return false
	}

	switch value[0] {
	case '{', '[', '"':
		return json.Valid([]byte(value))
	default:
		return false
	}
}
