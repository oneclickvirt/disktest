package disk

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestStandardFioScenarios(t *testing.T) {
	scenarios := StandardFioScenarios()
	if len(scenarios) != 8 {
		t.Fatalf("got %d scenarios", len(scenarios))
	}
	for _, scenario := range scenarios {
		if scenario.QueueDepth <= 0 || scenario.Jobs != 1 || scenario.RW == "randrw" {
			t.Fatalf("unsafe or mixed scenario: %+v", scenario)
		}
	}
}

func TestDeepFioScenariosIncludeATTOTransferSweep(t *testing.T) {
	scenarios := DeepFioScenarios()
	if len(scenarios) != len(StandardFioScenarios())+20 {
		t.Fatalf("deep scenarios = %d", len(scenarios))
	}
	want := map[string]bool{"atto-512b-read": false, "atto-64m-write": false}
	for _, scenario := range scenarios {
		if _, exists := want[scenario.ID]; exists {
			want[scenario.ID] = true
		}
		if scenario.Jobs != 1 || scenario.QueueDepth < 1 || scenario.RW == "randrw" {
			t.Fatalf("unsafe deep scenario: %+v", scenario)
		}
	}
	for id, found := range want {
		if !found {
			t.Fatalf("deep scenario %q missing", id)
		}
	}
}

func TestRunDeepFioMatrixRejectsUnsafeSizeBeforeExecution(t *testing.T) {
	result := RunDeepFioMatrix(context.Background(), MatrixConfig{Path: t.TempDir(), SizeBytes: 1 << 20})
	if result.Status != "unavailable" || result.Error == "" {
		t.Fatalf("unexpected deep result: %+v", result)
	}
}

func TestParseFioJSON(t *testing.T) {
	fixture := []byte(`{"jobs":[{"read":{"bw_bytes":1048576,"iops":256,"clat_ns":{"percentile":{"50.000000":1000,"95.000000":2000,"99.000000":3000}}},"write":{"bw":512,"iops":128,"clat_ns":{"percentile":{"50.000000":4000,"95.000000":5000,"99.000000":6000}}}}]}`)
	metrics, err := ParseFioJSON(fixture, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if len(metrics) != 2 || metrics[0].BandwidthBytesPerSecond != 1048576 || metrics[0].LatencyP95NS != 2000 || metrics[1].BandwidthBytesPerSecond != 512*1024 {
		t.Fatalf("unexpected metrics: %+v", metrics)
	}
}

func TestParseFioJSONRejectsMissingJobs(t *testing.T) {
	if _, err := ParseFioJSON([]byte(`{"jobs":[]}`), "empty"); err == nil {
		t.Fatal("expected missing jobs error")
	}
}

func TestEnsureMatrixSpaceRejectsRawDeviceAndSmallFile(t *testing.T) {
	if err := ensureMatrixSpace(t.TempDir(), 1<<20); err == nil {
		t.Fatal("expected small file rejection")
	}
	if err := ensureMatrixSpace("/dev", 16<<20); err == nil {
		t.Fatal("expected raw path rejection")
	}
}

func TestRunStandardFioMatrixRejectsUnsafeSizeBeforeExecution(t *testing.T) {
	result := RunStandardFioMatrix(context.Background(), MatrixConfig{Path: t.TempDir(), SizeBytes: 1 << 20})
	if result.Status != "unavailable" || result.Error == "" || result.DurationMS < 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRunFioMatrixCancelsContextAwareProvider(t *testing.T) {
	directory := t.TempDir()
	started := make(chan struct{})
	provider := func(ctx context.Context) (fioAcquisition, error) {
		close(started)
		<-ctx.Done()
		return fioAcquisition{}, ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	resultChannel := make(chan MatrixResult, 1)
	go func() {
		resultChannel <- runFioMatrixWithProvider(ctx, MatrixConfig{
			Path: directory, SizeBytes: 16 << 20, MaxDuration: time.Minute,
		}, StandardFioScenarios(), time.Minute, provider)
	}()
	<-started
	cancel()
	select {
	case result := <-resultChannel:
		if result.Status != "canceled" || result.Error == "" {
			t.Fatalf("unexpected canceled result: %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked fio provider prevented cancellation")
	}
	assertDirectoryEmpty(t, directory)
}

func TestRunFioMatrixReportsDeadlineWhileProviderBlocks(t *testing.T) {
	directory := t.TempDir()
	providerStarted := make(chan struct{})
	provider := func(ctx context.Context) (fioAcquisition, error) {
		close(providerStarted)
		<-ctx.Done()
		return fioAcquisition{}, ctx.Err()
	}
	resultChannel := make(chan MatrixResult, 1)
	go func() {
		resultChannel <- runFioMatrixWithProvider(context.Background(), MatrixConfig{
			Path: directory, SizeBytes: 16 << 20, MaxDuration: 20 * time.Millisecond,
		}, StandardFioScenarios(), time.Minute, provider)
	}()
	<-providerStarted
	select {
	case result := <-resultChannel:
		if result.Status != "timeout" || result.Error == "" {
			t.Fatalf("unexpected timeout result: %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("blocked fio provider exceeded matrix deadline")
	}
	assertDirectoryEmpty(t, directory)
}

func TestFindSystemFIOReportsUnavailable(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := findSystemFIO(context.Background()); err == nil {
		t.Fatal("expected missing system fio error")
	}
}

func TestFindFIOFallsBackToEmbeddedCommandAndCleansIt(t *testing.T) {
	directory := t.TempDir()
	script := filepath.Join(directory, "embedded-fio")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	originalGet, originalClean := getEmbeddedFIO, cleanEmbeddedFIO
	cleaned := false
	getEmbeddedFIO = func() (string, string, error) { return script, script, nil }
	cleanEmbeddedFIO = func(path string) error {
		cleaned = path == script
		return nil
	}
	defer func() { getEmbeddedFIO, cleanEmbeddedFIO = originalGet, originalClean }()
	acquired, err := findFIO(context.Background())
	if err != nil || len(acquired.Command) != 1 || acquired.Command[0] != script || acquired.Cleanup == nil {
		t.Fatalf("unexpected embedded acquisition: %+v err=%v", acquired, err)
	}
	if err := acquired.Cleanup(); err != nil || !cleaned {
		t.Fatalf("embedded cleanup failed: cleaned=%v err=%v", cleaned, err)
	}
}

func TestFindSystemFIOProbeHonorsDeadline(t *testing.T) {
	directory := t.TempDir()
	script := filepath.Join(directory, "fio")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nwhile :; do :; done\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err := findSystemFIO(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("findSystemFIO error = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("fio provider probe ignored deadline: %s", elapsed)
	}
}

func TestRunFioMatrixCleansRegularTestAndEngineProbeFiles(t *testing.T) {
	directory := t.TempDir()
	var cleanupCalls atomic.Int32
	var testPath, probePath string
	provider := func(context.Context) (fioAcquisition, error) {
		return fioAcquisition{
			Command: []string{"fixture-fio"},
			Cleanup: func() error {
				cleanupCalls.Add(1)
				return nil
			},
		}, nil
	}
	runner := func(ctx context.Context, command []string) ([]byte, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name, filename := commandArgument(command, "--name="), commandArgument(command, "--filename=")
		if filename == "" {
			t.Fatalf("fixture command has no filename: %#v", command)
		}
		info, err := os.Stat(filename)
		if err != nil {
			t.Fatalf("fixture file is unavailable during command: %v", err)
		}
		if !info.Mode().IsRegular() {
			t.Fatalf("fixture path is not a regular file: %s (%s)", filename, info.Mode())
		}
		if name == "engine-check" {
			probePath = filename
			return nil, nil
		}
		testPath = filename
		return []byte(`{"jobs":[{"read":{"bw_bytes":1048576,"iops":256,"clat_ns":{"percentile":{"50.000000":1000,"95.000000":2000,"99.000000":3000}}}}]}`), nil
	}

	result := runFioMatrixWithDeps(context.Background(), MatrixConfig{
		Path: directory, SizeBytes: 16 << 20, Runtime: time.Second, MaxDuration: 5 * time.Second,
	}, []FioScenario{{ID: "fixture-read", RW: "read", BlockSize: "4k", QueueDepth: 1, Jobs: 1}}, time.Minute, provider, runner)
	if result.Status != "ok" || len(result.Metrics) != 1 {
		t.Fatalf("unexpected fixture result: %+v", result)
	}
	if testPath == "" || probePath == "" || testPath == probePath {
		t.Fatalf("temporary paths were not observed: test=%q probe=%q", testPath, probePath)
	}
	for _, path := range []string{testPath, probePath} {
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("temporary file was not removed: path=%s err=%v", path, err)
		}
	}
	if cleanupCalls.Load() != 1 {
		t.Fatalf("provider cleanup calls = %d, want 1", cleanupCalls.Load())
	}
	assertDirectoryEmpty(t, directory)
}

func TestRunFioMatrixCancellationStopsProbeAndCleansFiles(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		newContext func() (context.Context, context.CancelFunc)
		wantStatus string
	}{
		{
			name: "canceled", wantStatus: "canceled",
			newContext: func() (context.Context, context.CancelFunc) {
				return context.WithCancel(context.Background())
			},
		},
		{
			name: "deadline", wantStatus: "timeout",
			newContext: func() (context.Context, context.CancelFunc) {
				return context.WithTimeout(context.Background(), 20*time.Millisecond)
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			directory := t.TempDir()
			ctx, cancel := testCase.newContext()
			defer cancel()
			started := make(chan struct{})
			var cleanupCalls atomic.Int32
			var matrixCalls atomic.Int32
			var testPath, probePath string
			provider := func(context.Context) (fioAcquisition, error) {
				return fioAcquisition{Command: []string{"fixture-fio"}, Cleanup: func() error {
					cleanupCalls.Add(1)
					return nil
				}}, nil
			}
			runner := func(runCtx context.Context, command []string) ([]byte, error) {
				filename := commandArgument(command, "--filename=")
				if commandArgument(command, "--name=") == "engine-check" {
					probePath = filename
					testPath = firstTemporaryPath(directory, ".goecs-fio-")
					close(started)
					<-runCtx.Done()
					return nil, runCtx.Err()
				}
				matrixCalls.Add(1)
				return nil, errors.New("matrix command started after context stopped")
			}
			resultChannel := make(chan MatrixResult, 1)
			go func() {
				resultChannel <- runFioMatrixWithDeps(ctx, MatrixConfig{
					Path: directory, SizeBytes: 16 << 20, Runtime: time.Second, MaxDuration: time.Minute,
				}, StandardFioScenarios(), time.Minute, provider, runner)
			}()
			<-started
			if testCase.wantStatus == "canceled" {
				cancel()
			}
			select {
			case result := <-resultChannel:
				if result.Status != testCase.wantStatus || result.Error == "" {
					t.Fatalf("unexpected stopped result: %+v", result)
				}
			case <-time.After(time.Second):
				t.Fatal("fio probe ignored context stop")
			}
			for _, path := range []string{testPath, probePath} {
				if path == "" {
					t.Fatal("expected temporary path was not observed")
				}
				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("temporary file was not removed: path=%s err=%v", path, err)
				}
			}
			if cleanupCalls.Load() != 1 {
				t.Fatalf("provider cleanup calls = %d, want 1", cleanupCalls.Load())
			}
			if matrixCalls.Load() != 0 {
				t.Fatalf("matrix command started %d times after context stopped", matrixCalls.Load())
			}
		})
	}
}

func TestRunFioMatrixCleansProviderAfterAcquisitionError(t *testing.T) {
	var cleanupCalls atomic.Int32
	result := runFioMatrixWithProvider(context.Background(), MatrixConfig{
		Path: t.TempDir(), SizeBytes: 16 << 20, MaxDuration: time.Second,
	}, StandardFioScenarios(), time.Minute, func(context.Context) (fioAcquisition, error) {
		return fioAcquisition{Cleanup: func() error {
			cleanupCalls.Add(1)
			return nil
		}}, errors.New("fixture acquisition failed")
	})
	if result.Status != "unavailable" || cleanupCalls.Load() != 1 {
		t.Fatalf("provider acquisition cleanup failed: result=%+v cleanup=%d", result, cleanupCalls.Load())
	}
}

func commandArgument(command []string, prefix string) string {
	for _, argument := range command {
		if strings.HasPrefix(argument, prefix) {
			return strings.TrimPrefix(argument, prefix)
		}
	}
	return ""
}

func firstTemporaryPath(directory, prefix string) string {
	entries, _ := os.ReadDir(directory)
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) && !strings.HasPrefix(entry.Name(), ".goecs-fio-engine-") {
			return filepath.Join(directory, entry.Name())
		}
	}
	return ""
}

func assertDirectoryEmpty(t *testing.T, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary directory is not clean: %#v", entries)
	}
}
