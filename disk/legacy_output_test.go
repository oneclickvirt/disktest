package disk

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/shirou/gopsutil/disk"
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

func TestLegacyFioArgsRunToConfiguredDataSize(t *testing.T) {
	for _, buffered := range []bool{false, true} {
		args := legacyFioArgs("libaio", "4k", "1G", "/tmp/test.fio", buffered)
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "--runtime=") || strings.Contains(joined, "--time_based") {
			t.Fatalf("legacy FIO still has a fixed runtime: %q", joined)
		}
		wantDirect := "--direct=1"
		if buffered {
			wantDirect = "--direct=0"
		}
		if !strings.Contains(joined, wantDirect) {
			t.Fatalf("legacy FIO direct mode missing %q: %q", wantDirect, joined)
		}
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

func TestLegacyResultUsableRequiresMeasuredTransferRate(t *testing.T) {
	for _, output := range []string{
		"", "Test Path Block Direct Write(IOPS) Direct Read(IOPS)\n",
		"Unable to create test path\n", "/ 4k Write failed Read failed\n",
	} {
		if LegacyResultUsable(output) {
			t.Fatalf("failure-only legacy output was considered usable: %q", output)
		}
	}
	for _, output := range []string{
		"/ 4k 12.3 MB/s(1) 9.8 MB/s(2)\n",
		"/ 4k Write failed 1.2 GiB/s(2)\n",
	} {
		if !LegacyResultUsable(output) {
			t.Fatalf("measured legacy output was rejected: %q", output)
		}
	}
}

func TestLegacyHeadersUseNormalizedEnglish(t *testing.T) {
	if got := generateFioTestHeader(" EN ", nil); !strings.Contains(got, "Test Path") {
		t.Fatalf("normalized English FIO header = %q", got)
	}
	if got := generateDDTestHeader(" EN ", nil); !strings.Contains(got, "Test Path") {
		t.Fatalf("normalized English DD header = %q", got)
	}
}

func TestVirtioPartitionIsKeptForMultiDiskBenchmarks(t *testing.T) {
	mountPoint := t.TempDir()
	parts := []disk.PartitionStat{{Device: "/dev/vda1", Mountpoint: mountPoint, Fstype: "ext4"}}

	pathInfo := testPathsFromPartitions(parts)
	if len(pathInfo.MountPoints) != 1 || pathInfo.MountPoints[0] != mountPoint || pathInfo.Devices[0] != "/dev/vda1" {
		t.Fatalf("virtio root partition was filtered: %#v", pathInfo)
	}
}

func TestEmptyMultiDiskDiscoveryFallsBackToFirstWritablePath(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	writable := t.TempDir()

	pathInfo := ensureLegacyTestPaths(TestPathInfo{}, missing, writable)
	if len(pathInfo.MountPoints) != 1 || pathInfo.MountPoints[0] != writable || pathInfo.Devices[0] != writable {
		t.Fatalf("empty discovery did not use writable fallback: %#v", pathInfo)
	}
	if _, err := os.Stat(filepath.Join(writable, ".temp_write_check")); !os.IsNotExist(err) {
		t.Fatalf("writability probe left a temporary file: %v", err)
	}
}

func TestExistingMultiDiskDiscoveryDoesNotUseFallback(t *testing.T) {
	original := TestPathInfo{Devices: []string{"/dev/sda1"}, MountPoints: []string{"/data"}}
	got := ensureLegacyTestPaths(original, t.TempDir())
	if len(got.MountPoints) != 1 || got.MountPoints[0] != "/data" || got.Devices[0] != "/dev/sda1" {
		t.Fatalf("existing discovery was replaced: %#v", got)
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

func TestNormalizeDDCommandDoesNotRequireSudo(t *testing.T) {
	executable, args, err := normalizeDDCommand("sudo /opt/bin/dd --status=none")
	if err != nil {
		t.Fatalf("normalizeDDCommand returned error: %v", err)
	}
	if executable != "/opt/bin/dd" || strings.Join(args, " ") != "--status=none" {
		t.Fatalf("normalized command = %q %q", executable, args)
	}
}

func TestDDDirectIOFlagUsesInputFlagForReads(t *testing.T) {
	if got := ddDirectIOFlag("linux", "/dev/zero", "/dev/zero"); got != "oflag=direct" {
		t.Fatalf("write direct flag = %q", got)
	}
	if got := ddDirectIOFlag("linux", "/tmp/test.file", "/dev/zero"); got != "iflag=direct" {
		t.Fatalf("read direct flag = %q", got)
	}
	if got := ddDirectIOFlag("darwin", "/tmp/test.file", "/dev/zero"); got != "" {
		t.Fatalf("darwin direct flag = %q", got)
	}
}
