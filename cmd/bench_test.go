package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/xueqianLu/rpcduel/internal/bench"
)

func TestRenderBenchTable(t *testing.T) {
	output := renderBenchTable([]bench.Summary{
		{
			Endpoint:          "rpc-a",
			Requests:          10,
			Successes:         9,
			Failures:          1,
			RPS:               123.45,
			P95:               100 * time.Millisecond,
			P99:               200 * time.Millisecond,
			ErrorDistribution: map[string]int{"http_503": 1},
		},
	})

	for _, want := range []string{
		"ENDPOINT",
		"rpc-a",
		"123.45",
		"100ms",
		"200ms",
		"http_503=1",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("renderBenchTable() missing %q in output:\n%s", want, output)
		}
	}
}
