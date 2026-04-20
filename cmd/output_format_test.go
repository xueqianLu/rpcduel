package cmd

import "testing"

func TestValidateOutputFormat(t *testing.T) {
	for _, tc := range []struct {
		name    string
		in      string
		wantErr bool
	}{
		{"text", "text", false},
		{"json", "json", false},
		{"empty", "", true},
		{"yaml", "yaml", true},
		{"uppercase", "JSON", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateOutputFormat(tc.in)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q", tc.in)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.in, err)
			}
		})
	}
}
