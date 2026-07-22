package disk

import (
	"context"
	"path/filepath"
	"strings"
	"time"
)

type MultiPathResult struct {
	SchemaVersion string         `json:"schema_version"`
	Status        string         `json:"status"`
	Paths         []MatrixResult `json:"paths"`
	Error         string         `json:"error,omitempty"`
}

func RunDeepMultiPathMatrix(ctx context.Context, paths []string, config MatrixConfig) MultiPathResult {
	if ctx == nil {
		ctx = context.Background()
	}
	result := MultiPathResult{SchemaVersion: "goecs.disk/deep-multi-v1", Status: "skipped", Paths: []MatrixResult{}}
	seen := make(map[string]struct{})
	for _, path := range paths {
		absolute, err := filepath.Abs(strings.TrimSpace(path))
		if err != nil || absolute == "" {
			continue
		}
		if _, exists := seen[absolute]; exists {
			continue
		}
		seen[absolute] = struct{}{}
		if err := ctx.Err(); err != nil {
			result.Status, result.Error = matrixStopStatus(err), stableMatrixError(err)
			return result
		}
		pathConfig := config
		pathConfig.Path = absolute
		pathConfig.MaxDuration = min(config.MaxDuration, 3*time.Minute)
		result.Paths = append(result.Paths, RunDeepFioMatrix(ctx, pathConfig))
	}
	if len(result.Paths) == 0 {
		result.Error = "no explicit deep disk paths configured"
		return result
	}
	ok := 0
	for _, pathResult := range result.Paths {
		if pathResult.Status == "ok" {
			ok++
		}
	}
	if ok == len(result.Paths) {
		result.Status = "ok"
	} else if ok > 0 {
		result.Status = "partial"
	} else {
		result.Status = "unavailable"
	}
	return result
}
