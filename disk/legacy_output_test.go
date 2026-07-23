package disk

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRenderLegacyResultsDoesNotEmitHeaderForEmptyBlocks(t *testing.T) {
	if got := renderLegacyResults("en", []string{"", "\n", "  \t"}, generateFioTestHeader); got != "" {
		t.Fatalf("empty result blocks produced output: %q", got)
	}
}

func TestRenderLegacyResultsKeepsPartialBlocksAndUsesOneHeader(t *testing.T) {
	row := "/                 4k        1.00 MB/s(1)       2.00 MB/s(2)       3.00 MB/s(3)\n"
	got := renderLegacyResults("en", []string{"", row, "\n"}, generateFioTestHeader)
	if got == "" || strings.Count(got, "Test Path") != 1 || !strings.Contains(got, row) {
		t.Fatalf("partial legacy result was not rendered: %q", got)
	}
}

func TestProcessFioOutputParsesTerseOutputFromEitherStream(t *testing.T) {
	fields := make([]string, 49)
	fields[0] = "rand_rw_4k"
	fields[6] = "1024"
	fields[7] = "100"
	fields[47] = "2048"
	fields[48] = "200"
	got := processFioOutput(strings.Join(fields, ";"), "4k", "/")
	if got == "" || !strings.Contains(got, "/                 4k") {
		t.Fatalf("terse fio output was not rendered: %q", got)
	}
}

func TestProcessFioOutputRejectsNonResultOutput(t *testing.T) {
	if got := processFioOutput("fio warning: no terse result", "4k", "/"); got != "" {
		t.Fatalf("unexpected row from non-result output: %q", got)
	}
}

func TestLocalizedTextUsesEnglishForNormalizedLanguage(t *testing.T) {
	if got := localizedText(" EN ", "写入失败", "Write failed"); got != "Write failed" {
		t.Fatalf("localized text = %q", got)
	}
	if got := localizedText("zh", "写入失败", "Write failed"); got != "写入失败" {
		t.Fatalf("localized text = %q", got)
	}
}

func TestDDTestContextStopsBeforeStartingCanceledBenchmark(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	started := time.Now()
	if got := DDTestContext(ctx, "en", false, ""); got != "" {
		t.Fatalf("canceled DD benchmark returned output: %q", got)
	}
	if elapsed := time.Since(started); elapsed > 100*time.Millisecond {
		t.Fatalf("canceled DD benchmark returned too slowly: %s", elapsed)
	}
}

func TestSleepContextStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if sleepContext(ctx, time.Second) {
		t.Fatal("canceled sleep reported completion")
	}
}

func TestParseResultDDEnglishGNUAndBusyBoxOutput(t *testing.T) {
	fixtures := []string{
		"104857600 bytes (105 MB, 100 MiB) copied, 0.050 s, 2.1 GB/s",
		"104857600 bytes (100.0MB) copied, 0.050 seconds, 2.1GB/s",
	}
	for _, fixture := range fixtures {
		got := parseResultDD(fixture, "25600")
		if !strings.Contains(got, "2.1 GB/s") || !strings.Contains(got, "IOPS") {
			t.Fatalf("English dd output was not parsed: input=%q output=%q", fixture, got)
		}
	}
}

func TestParseResultDDBSDOutput(t *testing.T) {
	got := parseResultDD("104857600 bytes transferred in 0.050 secs (2097152000 bytes/sec)", "25600")
	if !strings.Contains(got, "GB/s") || !strings.Contains(got, "IOPS") {
		t.Fatalf("BSD dd output was not parsed: %q", got)
	}
}
