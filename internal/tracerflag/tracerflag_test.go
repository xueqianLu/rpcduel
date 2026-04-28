// Copyright 2026 The rpcduel Authors
// SPDX-License-Identifier: Apache-2.0

package tracerflag

import (
	"reflect"
	"testing"
)

func TestBuildDefault(t *testing.T) {
	got, err := Build("", "")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{"tracer": "callTracer"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBuildExplicitTracer(t *testing.T) {
	got, err := Build("prestateTracer", `{"diffMode":true}`)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]interface{}{
		"tracer":       "prestateTracer",
		"tracerConfig": map[string]interface{}{"diffMode": true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestBuildDefaultKeyword(t *testing.T) {
	got, err := Build("default", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestBuildDefaultRejectsConfig(t *testing.T) {
	if _, err := Build("default", `{"x":1}`); err == nil {
		t.Error("expected error when combining --tracer=default with --tracer-config")
	}
}

func TestBuildBadJSON(t *testing.T) {
	if _, err := Build("callTracer", "{not json"); err == nil {
		t.Error("expected JSON parse error")
	}
}
