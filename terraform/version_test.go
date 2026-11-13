package terraform

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func indexServer(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "index.json"))
	assert.NilError(t, err)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/terraform/index.json" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(data)
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestResolveVersion(t *testing.T) {
	baseURL := indexServer(t)

	tests := []struct {
		name string
		spec string
		want string
	}{
		{"exact", "1.9.0", "1.9.0"},
		{"exact not in index is returned as-is", "1.2.3", "1.2.3"},
		{"latest picks newest stable", "latest", "1.10.3"},
		{"prefix minor", "1.9", "1.9.8"},
		{"prefix major", "1", "1.10.3"},
		{"prefix full minor line", "1.10", "1.10.3"},
		{"prefix matches whole components only", "1.1", "1.1.9"},
		{"prefix with trailing dot", "1.9.", "1.9.8"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveVersion(context.Background(), baseURL, tc.spec)
			assert.NilError(t, err)
			assert.Equal(t, got, tc.want)
		})
	}
}

func TestResolveVersionErrors(t *testing.T) {
	baseURL := indexServer(t)

	tests := []struct {
		name string
		spec string
	}{
		{"prefix with no match", "2"},
		{"prefix matching only a prerelease", "1.11"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ResolveVersion(context.Background(), baseURL, tc.spec)
			assert.ErrorContains(t, err, "no stable terraform version matches")
		})
	}
}

func TestResolveVersionExactSkipsServer(t *testing.T) {
	_, err := ResolveVersion(context.Background(), "http://127.0.0.1:0", "1.9.8")
	assert.NilError(t, err)
}
