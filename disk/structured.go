package disk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	embeddedfio "github.com/oneclickvirt/fio"
	gopsutildisk "github.com/shirou/gopsutil/disk"
)

type FioScenario struct {
	ID         string `json:"id"`
	RW         string `json:"rw"`
	BlockSize  string `json:"block_size"`
	QueueDepth int    `json:"queue_depth"`
	Jobs       int    `json:"jobs"`
}

type FioMetrics struct {
	ScenarioID              string  `json:"scenario_id"`
	Direction               string  `json:"direction"`
	BandwidthBytesPerSecond uint64  `json:"bandwidth_bytes_per_second"`
	IOPS                    float64 `json:"iops"`
	LatencyP50NS            uint64  `json:"latency_p50_ns"`
	LatencyP95NS            uint64  `json:"latency_p95_ns"`
	LatencyP99NS            uint64  `json:"latency_p99_ns"`
}

type MatrixConfig struct {
	Path        string
	SizeBytes   int64
	Runtime     time.Duration
	MaxDuration time.Duration
}

type MatrixResult struct {
	SchemaVersion string       `json:"schema_version"`
	Status        string       `json:"status"`
	Metrics       []FioMetrics `json:"metrics,omitempty"`
	DurationMS    int64        `json:"duration_ms"`
	Error         string       `json:"error,omitempty"`
}

type fioAcquisition struct {
	Command []string
	Cleanup func() error
}

type fioProvider func(context.Context) (fioAcquisition, error)

type fioCommandRunner func(context.Context, []string) ([]byte, error)

func StandardFioScenarios() []FioScenario {
	return []FioScenario{
		{ID: "4k-q1-read", RW: "randread", BlockSize: "4k", QueueDepth: 1, Jobs: 1},
		{ID: "4k-q1-write", RW: "randwrite", BlockSize: "4k", QueueDepth: 1, Jobs: 1},
		{ID: "4k-q32-read", RW: "randread", BlockSize: "4k", QueueDepth: 32, Jobs: 1},
		{ID: "4k-q32-write", RW: "randwrite", BlockSize: "4k", QueueDepth: 32, Jobs: 1},
		{ID: "1m-q1-read", RW: "read", BlockSize: "1m", QueueDepth: 1, Jobs: 1},
		{ID: "1m-q1-write", RW: "write", BlockSize: "1m", QueueDepth: 1, Jobs: 1},
		{ID: "1m-q8-read", RW: "read", BlockSize: "1m", QueueDepth: 8, Jobs: 1},
		{ID: "1m-q8-write", RW: "write", BlockSize: "1m", QueueDepth: 8, Jobs: 1},
	}
}

// DeepFioScenarios extends the standard random/sequential matrix with an
// ATTO-style sequential transfer-size sweep. It still writes only to one
// bounded temporary regular file selected by the caller.
func DeepFioScenarios() []FioScenario {
	result := append([]FioScenario(nil), StandardFioScenarios()...)
	for _, size := range []struct {
		id string
		bs string
	}{
		{id: "512b", bs: "512"}, {id: "2k", bs: "2k"},
		{id: "8k", bs: "8k"}, {id: "32k", bs: "32k"},
		{id: "128k", bs: "128k"}, {id: "512k", bs: "512k"},
		{id: "2m", bs: "2m"}, {id: "8m", bs: "8m"},
		{id: "32m", bs: "32m"}, {id: "64m", bs: "64m"},
	} {
		result = append(result,
			FioScenario{ID: "atto-" + size.id + "-read", RW: "read", BlockSize: size.bs, QueueDepth: 4, Jobs: 1},
			FioScenario{ID: "atto-" + size.id + "-write", RW: "write", BlockSize: size.bs, QueueDepth: 4, Jobs: 1},
		)
	}
	return result
}

func ParseFioJSON(data []byte, scenarioID string) ([]FioMetrics, error) {
	var document struct {
		Jobs []struct {
			Read  fioDirection `json:"read"`
			Write fioDirection `json:"write"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	if len(document.Jobs) == 0 {
		return nil, errors.New("fio JSON contains no jobs")
	}
	result := make([]FioMetrics, 0, 2)
	for _, direction := range []struct {
		name string
		get  func(int) fioDirection
	}{
		{name: "read", get: func(index int) fioDirection { return document.Jobs[index].Read }},
		{name: "write", get: func(index int) fioDirection { return document.Jobs[index].Write }},
	} {
		var merged FioMetrics
		merged.ScenarioID, merged.Direction = scenarioID, direction.name
		var active bool
		for index := range document.Jobs {
			value := direction.get(index)
			if value.IOPS == 0 && value.BandwidthBytes == 0 && value.BandwidthKiB == 0 {
				continue
			}
			active = true
			bandwidth := value.BandwidthBytes
			if bandwidth == 0 {
				bandwidth = value.BandwidthKiB * 1024
			}
			merged.BandwidthBytesPerSecond += bandwidth
			merged.IOPS += value.IOPS
			merged.LatencyP50NS = max(merged.LatencyP50NS, percentileFromMap(value.ClatNS.Percentile, 50))
			merged.LatencyP95NS = max(merged.LatencyP95NS, percentileFromMap(value.ClatNS.Percentile, 95))
			merged.LatencyP99NS = max(merged.LatencyP99NS, percentileFromMap(value.ClatNS.Percentile, 99))
		}
		if active {
			result = append(result, merged)
		}
	}
	if len(result) == 0 {
		return nil, errors.New("fio JSON contains no read or write metrics")
	}
	return result, nil
}

func RunStandardFioMatrix(ctx context.Context, config MatrixConfig) (result MatrixResult) {
	return runFioMatrix(ctx, config, StandardFioScenarios(), 60*time.Second)
}

func RunDeepFioMatrix(ctx context.Context, config MatrixConfig) (result MatrixResult) {
	return runFioMatrix(ctx, config, DeepFioScenarios(), 3*time.Minute)
}

func runFioMatrix(ctx context.Context, config MatrixConfig, scenarios []FioScenario, maximumDuration time.Duration) (result MatrixResult) {
	return runFioMatrixWithProvider(ctx, config, scenarios, maximumDuration, findFIO)
}

func runFioMatrixWithProvider(ctx context.Context, config MatrixConfig, scenarios []FioScenario, maximumDuration time.Duration, provider fioProvider) (result MatrixResult) {
	return runFioMatrixWithDeps(ctx, config, scenarios, maximumDuration, provider, runFIOCommand)
}

func runFioMatrixWithDeps(ctx context.Context, config MatrixConfig, scenarios []FioScenario, maximumDuration time.Duration, provider fioProvider, runner fioCommandRunner) (result MatrixResult) {
	if ctx == nil {
		ctx = context.Background()
	}
	if provider == nil {
		provider = findFIO
	}
	if runner == nil {
		runner = runFIOCommand
	}
	if config.Path == "" {
		config.Path = os.TempDir()
	}
	if config.SizeBytes <= 0 {
		config.SizeBytes = 256 << 20
	}
	if config.SizeBytes > 2<<30 {
		config.SizeBytes = 2 << 30
	}
	if config.Runtime <= 0 {
		config.Runtime = 5 * time.Second
	}
	if config.Runtime > 10*time.Second {
		config.Runtime = 10 * time.Second
	}
	if len(scenarios) == 0 {
		return MatrixResult{SchemaVersion: "goecs.disk/v1", Status: "unavailable", Error: "fio scenario list is empty"}
	}
	if maximumDuration <= 0 {
		maximumDuration = 60 * time.Second
	}
	if config.MaxDuration <= 0 || config.MaxDuration > maximumDuration {
		config.MaxDuration = maximumDuration
	}
	result = MatrixResult{SchemaVersion: "goecs.disk/v1", Status: "ok"}
	started := time.Now()
	defer func() { result.DurationMS = time.Since(started).Milliseconds() }()
	matrixCtx, cancel := context.WithTimeout(ctx, config.MaxDuration)
	defer cancel()
	if err := matrixCtx.Err(); err != nil {
		result.Status, result.Error = matrixStopStatus(err), stableMatrixError(err)
		return result
	}
	if err := ensureMatrixSpace(config.Path, config.SizeBytes); err != nil {
		result.Status, result.Error = "unavailable", stableTestPathError(err)
		return result
	}
	testFile, err := os.CreateTemp(config.Path, ".goecs-fio-*")
	if err != nil {
		result.Status, result.Error = "unavailable", stableTestPathError(err)
		return result
	}
	testPath := testFile.Name()
	_ = testFile.Close()
	defer os.Remove(testPath)
	acquired, err := provider(matrixCtx)
	if acquired.Cleanup != nil {
		defer func() { _ = acquired.Cleanup() }()
	}
	if err != nil {
		if matrixCtx.Err() != nil {
			result.Status = matrixStopStatus(matrixCtx.Err())
		} else {
			result.Status = "unavailable"
		}
		result.Error = stableMatrixError(err)
		return result
	}
	if len(acquired.Command) == 0 || strings.TrimSpace(acquired.Command[0]) == "" {
		result.Status, result.Error = "unavailable", "fio command is empty"
		return result
	}
	perScenarioRuntime := config.Runtime
	maximumPerScenario := config.MaxDuration / time.Duration(len(scenarios))
	if perScenarioRuntime > maximumPerScenario {
		perScenarioRuntime = maximumPerScenario
	}
	ioEngine := selectMatrixIOEngine(matrixCtx, acquired.Command, config.Path, runner)
	for _, scenario := range scenarios {
		if err := matrixCtx.Err(); err != nil {
			result.Status, result.Error = matrixStopStatus(err), stableMatrixError(err)
			return result
		}
		command := append([]string{}, acquired.Command...)
		args := make([]string, 0, 12)
		args = append(args,
			"--name="+scenario.ID, "--ioengine="+ioEngine, "--rw="+scenario.RW,
			"--bs="+scenario.BlockSize, fmt.Sprintf("--iodepth=%d", scenario.QueueDepth),
			fmt.Sprintf("--numjobs=%d", scenario.Jobs), fmt.Sprintf("--size=%d", config.SizeBytes),
			fmt.Sprintf("--runtime=%d", max(int(perScenarioRuntime.Seconds()), 1)), "--time_based=1",
			"--direct=1", "--filename="+testPath, "--group_reporting=1", "--output-format=json",
		)
		command = append(command, args...)
		output, runErr := runner(matrixCtx, command)
		if runErr != nil {
			if matrixCtx.Err() != nil {
				result.Status, result.Error = matrixStopStatus(matrixCtx.Err()), stableMatrixError(matrixCtx.Err())
			} else {
				result.Status, result.Error = "error", "fio_failed"
			}
			return result
		}
		metrics, parseErr := ParseFioJSON(output, scenario.ID)
		if parseErr != nil {
			result.Status, result.Error = "error", "invalid_fio_output"
			return result
		}
		result.Metrics = append(result.Metrics, metrics...)
	}
	return result
}

var getEmbeddedFIO = embeddedfio.GetFIO
var cleanEmbeddedFIO = embeddedfio.CleanFio

func findFIO(ctx context.Context) (fioAcquisition, error) {
	if acquired, err := findSystemFIO(ctx); err == nil {
		return acquired, nil
	} else if ctx.Err() != nil {
		return fioAcquisition{}, ctx.Err()
	}
	command, temporaryPath, err := getEmbeddedFIO()
	if err != nil {
		return fioAcquisition{}, fmt.Errorf("embedded fio is unavailable: %w", err)
	}
	parts := splitCommand(command)
	cleanup := func() error { return cleanEmbeddedFIO(temporaryPath) }
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		_ = cleanup()
		return fioAcquisition{}, errors.New("embedded fio command is empty")
	}
	if _, err := runFIOCommand(ctx, append(append([]string(nil), parts...), "--help")); err != nil {
		_ = cleanup()
		if ctx.Err() != nil {
			return fioAcquisition{}, ctx.Err()
		}
		return fioAcquisition{}, fmt.Errorf("embedded fio probe failed: %w", err)
	}
	return fioAcquisition{Command: parts, Cleanup: cleanup}, nil
}

func findSystemFIO(ctx context.Context) (fioAcquisition, error) {
	if err := ctx.Err(); err != nil {
		return fioAcquisition{}, err
	}
	path, err := exec.LookPath("fio")
	if err != nil {
		return fioAcquisition{}, fmt.Errorf("system fio is unavailable: %w", err)
	}
	if _, err := runFIOCommand(ctx, []string{path, "--help"}); err != nil {
		if ctx.Err() != nil {
			return fioAcquisition{}, ctx.Err()
		}
		return fioAcquisition{}, fmt.Errorf("system fio probe failed: %w", err)
	}
	return fioAcquisition{Command: []string{path}}, nil
}

func runFIOCommand(ctx context.Context, command []string) ([]byte, error) {
	if len(command) == 0 || strings.TrimSpace(command[0]) == "" {
		return nil, errors.New("fio command is empty")
	}
	return exec.CommandContext(ctx, command[0], command[1:]...).Output()
}

func matrixStopStatus(err error) string {
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	return "timeout"
}

func stableMatrixError(err error) string {
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return "fio_unavailable"
}

func stableTestPathError(err error) string {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "test_path_not_found"
	case errors.Is(err, os.ErrPermission):
		return "test_path_permission_denied"
	}
	message := strings.ToLower(err.Error())
	switch {
	case strings.Contains(message, "insufficient free space"):
		return "insufficient_space"
	case strings.Contains(message, "not a directory"):
		return "test_path_not_directory"
	case strings.Contains(message, "raw device"):
		return "raw_device_forbidden"
	case strings.Contains(message, "at least 16 mib"):
		return "unsafe_test_size"
	default:
		return "test_path_unavailable"
	}
}

func selectMatrixIOEngine(ctx context.Context, commandParts []string, directory string, runner fioCommandRunner) string {
	engines := []string{"psync"}
	switch runtime.GOOS {
	case "linux":
		engines = []string{"io_uring", "libaio", "posixaio", "psync"}
	case "darwin", "freebsd":
		engines = []string{"posixaio", "psync"}
	case "windows":
		engines = []string{"windowsaio", "psync"}
	}
	probe, err := os.CreateTemp(directory, ".goecs-fio-engine-*")
	if err != nil {
		return "psync"
	}
	probePath := probe.Name()
	probe.Close()
	defer os.Remove(probePath)
	for _, engine := range engines {
		if ctx.Err() != nil {
			return "psync"
		}
		command := append([]string{}, commandParts...)
		args := make([]string, 0, 7)
		args = append(args, "--name=engine-check", "--ioengine="+engine, "--rw=read", "--size=1M", "--runtime=1", "--filename="+probePath, "--output-format=json")
		command = append(command, args...)
		if _, err := runner(ctx, command); err == nil {
			return engine
		}
	}
	return "psync"
}

func ensureMatrixSpace(path string, requested int64) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("fio path is not a directory")
	}
	probe, err := os.CreateTemp(path, ".goecs-write-probe-*")
	if err != nil {
		return err
	}
	probeName := probe.Name()
	probe.Close()
	os.Remove(probeName)
	if requested < 16<<20 {
		return errors.New("fio size must be at least 16 MiB")
	}
	if filepath.Clean(path) == "/dev" {
		return errors.New("raw device paths are not allowed")
	}
	usage, err := gopsutildisk.Usage(path)
	if err != nil {
		return err
	}
	reserve := uint64(512 << 20)
	if usage.Free <= uint64(requested)+reserve {
		return errors.New("insufficient free space for fio test and safety reserve")
	}
	return nil
}

type fioDirection struct {
	BandwidthBytes uint64  `json:"bw_bytes"`
	BandwidthKiB   uint64  `json:"bw"`
	IOPS           float64 `json:"iops"`
	ClatNS         struct {
		Percentile map[string]uint64 `json:"percentile"`
	} `json:"clat_ns"`
}

func percentileFromMap(values map[string]uint64, target float64) uint64 {
	if len(values) == 0 {
		return 0
	}
	type point struct {
		percentile float64
		value      uint64
	}
	points := make([]point, 0, len(values))
	for raw, value := range values {
		parsed, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err == nil {
			points = append(points, point{percentile: parsed, value: value})
		}
	}
	if len(points) == 0 {
		return 0
	}
	sort.Slice(points, func(i, j int) bool { return points[i].percentile < points[j].percentile })
	for _, point := range points {
		if point.percentile >= target {
			return point.value
		}
	}
	return points[len(points)-1].value
}
