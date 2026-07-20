package main

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestParseCLIOptions(t *testing.T) {
	opts, err := parseCLI([]string{"--structured", "--deep", "--duration", "2s", "--timeout", "30s", "--size", "1048576", "-p", "/tmp/TestPath"})
	if err != nil {
		t.Fatalf("parseCLI returned error: %v", err)
	}
	if !opts.jsonOutput || !opts.deep || opts.runtime != 2*time.Second || opts.timeout != 30*time.Second || opts.sizeBytes != 1048576 || opts.path != "/tmp/TestPath" {
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

func TestCLIActionPrioritizesHelpAndVersion(t *testing.T) {
	if got := selectCLIAction(cliOptions{help: true, version: true, jsonOutput: true}); got != "help" {
		t.Fatalf("help action = %q", got)
	}
	if got := selectCLIAction(cliOptions{version: true, jsonOutput: true}); got != "version" {
		t.Fatalf("version action = %q", got)
	}
}
