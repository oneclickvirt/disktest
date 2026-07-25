package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestParseCLIOptions(t *testing.T) {
	opts, err := parseCLI([]string{"--structured", "--deep", "--duration", "2s", "--timeout", "30s", "--size", "16777216", "-p", "/tmp/TestPath"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	if !opts.jsonOutput || !opts.deep || opts.runtime != 2*time.Second || opts.timeout != 30*time.Second || opts.sizeBytes != 16777216 || opts.path != "/tmp/TestPath" {
		t.Fatalf("unexpected options: %#v", opts)
	}
}

func TestHelpRetainsLegacyFlags(t *testing.T) {
	var output bytes.Buffer
	newFlagSet(&cliOptions{}, &output).PrintDefaults()
	for _, legacy := range []string{"-d string", "-h", "-l string", "-m string", "-p string", "-log", "-v"} {
		if !strings.Contains(output.String(), legacy) {
			t.Fatalf("help is missing legacy flag %q: %s", legacy, output.String())
		}
	}
}

func TestParseCLIRejectsNegativeTimeout(t *testing.T) {
	if _, err := parseCLI([]string{"--timeout", "-1s"}); err == nil {
		t.Fatal("expected negative timeout to be rejected")
	}
}

func TestParseCLIRejectsInvalidAndIgnoredOptions(t *testing.T) {
	for _, args := range [][]string{
		{"-l", "fr"},
		{"-m", "unknown"},
		{"-d", "all"},
		{"-p", ""},
		{"--duration", "1s"},
		{"--structured", "-m", "fio"},
		{"--structured", "--size", "1048576"},
		{"unexpected"},
	} {
		if _, err := parseCLI(args); err == nil {
			t.Fatalf("expected arguments %v to be rejected", args)
		}
	}
}

func TestParseCLIAllowsCallerOwnedLongStructuredLimits(t *testing.T) {
	opts, err := parseCLI([]string{"--structured", "--duration", "30s", "--timeout", "15m"})
	if err != nil {
		t.Fatalf("long explicit limits were rejected: %v", err)
	}
	if opts.runtime != 30*time.Second || opts.timeout != 15*time.Minute {
		t.Fatalf("long explicit limits were changed: %#v", opts)
	}
}

func TestParseCLIAllowsUnlimitedStructuredLimits(t *testing.T) {
	opts, err := parseCLI([]string{"--structured", "--duration", "0", "--timeout", "0"})
	if err != nil {
		t.Fatalf("zero unlimited limits were rejected: %v", err)
	}
	if !opts.runtimeSet || !opts.timeoutSet || opts.runtime != 0 || opts.timeout != 0 {
		t.Fatalf("zero unlimited limits were changed: %#v", opts)
	}
}

func TestParseCLIHelpAndVersionIgnoreOtherInvalidValues(t *testing.T) {
	for _, args := range [][]string{{"-h", "-l", "fr"}, {"-v", "-m", "unknown"}} {
		if _, err := parseCLI(args); err != nil {
			t.Fatalf("help/version arguments %v returned %v", args, err)
		}
	}
}

func TestCLIActionPrioritizesHelpAndVersion(t *testing.T) {
	if got := selectCLIAction(cliOptions{help: true, version: true, jsonOutput: true}); got != "help" {
		t.Fatalf("help action = %q", got)
	}
	if got := selectCLIAction(cliOptions{version: true, jsonOutput: true}); got != "version" {
		t.Fatalf("version action = %q", got)
	}
}

func TestFailedLegacyDiskTextRetainsFailureAndExplainsFallback(t *testing.T) {
	got := failedLegacyDiskText("en", "", "/ 4k Write failed Read failed\n")
	if !strings.Contains(got, "Write failed") || !strings.Contains(got, "DD and FIO returned no usable benchmark data.") {
		t.Fatalf("English fallback detail = %q", got)
	}
	got = failedLegacyDiskText("zh", "创建测试路径失败\n")
	if !strings.Contains(got, "创建测试路径失败") || !strings.Contains(got, "均未返回可用的性能数据") {
		t.Fatalf("Chinese fallback detail = %q", got)
	}
}

func TestRunLegacyDiskFallbackAttemptsSecondMethod(t *testing.T) {
	var primaryCalls, fallbackCalls int
	primary := func(string, bool, string) string {
		primaryCalls++
		return "FIO failed\n"
	}
	fallback := func(string, bool, string) string {
		fallbackCalls++
		return "/ 4k 1.00 MB/s(1) 2.00 MB/s(2)\n"
	}
	got := runLegacyDiskFallback(" EN ", false, "/tmp", primary, fallback, "切换至DD", "switched to DD")
	if primaryCalls != 1 || fallbackCalls != 1 {
		t.Fatalf("fallback call counts = primary:%d fallback:%d", primaryCalls, fallbackCalls)
	}
	if !strings.Contains(got, "switched to DD") || !strings.Contains(got, "MB/s") {
		t.Fatalf("fallback result = %q", got)
	}
}

func TestRunLegacyDiskFallbackSkipsSecondMethodWhenPrimaryHasData(t *testing.T) {
	var fallbackCalls int
	primary := func(string, bool, string) string { return "/ 4k 1.00 MB/s(1) 2.00 MB/s(2)\n" }
	fallback := func(string, bool, string) string {
		fallbackCalls++
		return "should not run"
	}
	got := runLegacyDiskFallback("zh", false, "", primary, fallback, "切换", "switched")
	if fallbackCalls != 0 || !strings.Contains(got, "MB/s") {
		t.Fatalf("primary result/fallback calls = %q/%d", got, fallbackCalls)
	}
}
