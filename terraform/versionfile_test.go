package terraform

import (
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func writeVersionFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".terraform-version")
	assert.NilError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func TestReadVersionFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"trailing newline", "1.14.7\n", "1.14.7"},
		{"no newline", "1.14.7", "1.14.7"},
		{"surrounding whitespace", "\n  1.14.7 \r\n\n", "1.14.7"},
		{"comments", "# pinned for the provider\n1.14.7 # bumped by Renovate\n", "1.14.7"},
		{"prefix", "1.9\n", "1.9"},
		{"latest", "latest\n", "latest"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ReadVersionFile(writeVersionFile(t, tc.content))
			assert.NilError(t, err)
			assert.Equal(t, got, tc.want)
		})
	}
}

func TestReadVersionFileErrors(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"empty", "", "has no version in it"},
		{"only whitespace", " \n\n", "has no version in it"},
		{"only a comment", "# 1.14.7\n", "has no version in it"},
		{"two versions", "1.14.7\n1.9.8\n", "has more than one version in it: 1.14.7 1.9.8"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ReadVersionFile(writeVersionFile(t, tc.content))
			assert.ErrorContains(t, err, tc.want)
		})
	}
}

func TestReadVersionFileMissing(t *testing.T) {
	_, err := ReadVersionFile(filepath.Join(t.TempDir(), ".terraform-version"))
	assert.Assert(t, os.IsNotExist(err), "got %v", err)
}
