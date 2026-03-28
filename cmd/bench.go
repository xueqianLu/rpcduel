package cmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/xueqianLu/rpcduel/internal/bench"
	"github.com/xueqianLu/rpcduel/internal/dataset"
)

var (
	benchTargets     []string
	benchInput       string
	benchConcurrency int
	benchRequests    int
	benchTimeout     time.Duration
)

var benchCmd = &cobra.Command{
	Use:   "bench",
	Short: "Benchmark one or more RPC endpoints with dataset-driven traffic",
	RunE: func(cmd *cobra.Command, args []string) error {
		file, err := dataset.Load(benchInput)
		if err != nil {
			return err
		}

		requests := bench.BuildRequests(file)
		if len(requests) == 0 {
			return fmt.Errorf("dataset %s did not contain any benchmarkable records", benchInput)
		}

		ctx := context.Background()
		summaries := make([]bench.Summary, 0, len(benchTargets))
		for _, rawTarget := range benchTargets {
			provider, err := newProvider(rawTarget, benchTimeout)
			if err != nil {
				return err
			}

			summary, err := bench.Run(ctx, provider, requests, benchConcurrency, benchRequests)
			if err != nil {
				return err
			}

			summaries = append(summaries, summary)
		}

		if _, err := io.WriteString(cmd.OutOrStdout(), renderBenchTable(summaries)); err != nil {
			return fmt.Errorf("write bench table: %w", err)
		}
		return nil
	},
}

func init() {
	benchCmd.Flags().StringArrayVar(&benchTargets, "to", nil, "RPC target alias or URL (repeatable)")
	benchCmd.Flags().StringVar(&benchInput, "input", "dataset.json", "Dataset file to replay")
	benchCmd.Flags().IntVar(&benchConcurrency, "concurrency", 32, "Worker count")
	benchCmd.Flags().IntVar(&benchRequests, "requests", 0, "Total requests to send (0 = one pass over the dataset)")
	benchCmd.Flags().DurationVar(&benchTimeout, "timeout", 15*time.Second, "Per-request timeout")

	_ = benchCmd.MarkFlagRequired("to")
}

func renderBenchTable(summaries []bench.Summary) string {
	if len(summaries) == 0 {
		return ""
	}

	var buffer bytes.Buffer
	writer := tabwriter.NewWriter(&buffer, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "ENDPOINT\tREQUESTS\tSUCCESS\tFAILURES\tRPS\tP95\tP99\tERRORS")

	for _, summary := range summaries {
		fmt.Fprintf(
			writer,
			"%s\t%d\t%d\t%d\t%.2f\t%s\t%s\t%s\n",
			summary.Endpoint,
			summary.Requests,
			summary.Successes,
			summary.Failures,
			summary.RPS,
			summary.P95,
			summary.P99,
			formatErrorDistribution(summary.ErrorDistribution),
		)
	}

	_ = writer.Flush()
	return buffer.String()
}

func formatErrorDistribution(distribution map[string]int) string {
	if len(distribution) == 0 {
		return "none"
	}

	keys := make([]string, 0, len(distribution))
	for key := range distribution {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", key, distribution[key]))
	}
	return strings.Join(parts, ", ")
}
