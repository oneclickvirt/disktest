package disk

import (
	"context"
	"testing"
)

func TestRunDeepMultiPathMatrixSkipsWithoutExplicitPaths(t *testing.T) {
	result := RunDeepMultiPathMatrix(context.Background(), nil, MatrixConfig{})
	if result.Status != "skipped" || result.Error == "" || len(result.Paths) != 0 {
		t.Fatalf("unexpected empty multi-path result: %+v", result)
	}
}

func TestRunDeepMultiPathMatrixRejectsUnsafeSizePerPath(t *testing.T) {
	path := t.TempDir()
	result := RunDeepMultiPathMatrix(context.Background(), []string{path, path}, MatrixConfig{SizeBytes: 1 << 20})
	if len(result.Paths) != 1 || result.Paths[0].Status != "unavailable" || result.Status != "unavailable" {
		t.Fatalf("unexpected multi-path safety result: %+v", result)
	}
}
