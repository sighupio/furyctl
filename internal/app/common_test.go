package app_test

import (
	"os"
	"testing"
)

var furyConfig = map[string]interface{}{
	"apiVersion": "kfd.sighup.io/v1alpha2",
	"kind":       "EKSCluster",
	"spec": map[string]interface{}{
		"distributionVersion": "v1.24.7",
		"distribution":        map[string]interface{}{},
	},
}

func mkDirTemp(t *testing.T, prefix string) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", prefix)
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}

	return tmpDir
}

func rmDirTemp(t *testing.T, dir string) {
	t.Helper()

	if err := os.RemoveAll(dir); err != nil {
		t.Log(err)
	}
}
