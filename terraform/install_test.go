package terraform

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"gotest.tools/v3/assert"
)

func makeZip(t *testing.T, entryName, content string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(entryName)
	assert.NilError(t, err)
	_, err = w.Write([]byte(content))
	assert.NilError(t, err)
	assert.NilError(t, zw.Close())
	return buf.Bytes()
}

// releaseServer serves the zip at the release path and SHA256SUMS with the
// given contents, so a test can supply a deliberately wrong or incomplete sums
// file.
func releaseServer(t *testing.T, version, osName, arch string, archive []byte, sums string) string {
	t.Helper()
	zipName := fmt.Sprintf("terraform_%s_%s_%s.zip", version, osName, arch)
	base := "/terraform/" + version + "/"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case base + zipName:
			_, _ = w.Write(archive)
		case base + fmt.Sprintf("terraform_%s_SHA256SUMS", version):
			_, _ = w.Write([]byte(sums))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func sumsLine(archive []byte, filename string) string {
	sum := sha256.Sum256(archive)
	return fmt.Sprintf("%x  %s\n", sum, filename)
}

func TestDownload(t *testing.T) {
	const version, osName, arch = "1.9.8", "linux", "amd64"
	const script = "#!/bin/sh\necho terraform\n"
	archive := makeZip(t, "terraform", script)
	zipName := fmt.Sprintf("terraform_%s_%s_%s.zip", version, osName, arch)
	sums := "abc123  some-other-file\n" + sumsLine(archive, zipName)

	baseURL := releaseServer(t, version, osName, arch, archive, sums)
	dest := t.TempDir()

	path, err := Download(context.Background(), baseURL, version, osName, arch, dest)
	assert.NilError(t, err)
	assert.Equal(t, path, filepath.Join(dest, "terraform"))

	got, err := os.ReadFile(path)
	assert.NilError(t, err)
	assert.Equal(t, string(got), script)

	info, err := os.Stat(path)
	assert.NilError(t, err)
	assert.Assert(t, info.Mode().Perm()&0100 != 0, "binary should be executable")
}

func TestDownloadErrors(t *testing.T) {
	const version, osName, arch = "1.9.8", "linux", "amd64"
	zipName := fmt.Sprintf("terraform_%s_%s_%s.zip", version, osName, arch)

	tests := []struct {
		name    string
		archive []byte
		sums    func(archive []byte) string
		wantErr string
	}{
		{
			name:    "checksum mismatch",
			archive: makeZip(t, "terraform", "real"),
			sums:    func([]byte) string { return sumsLine(makeZip(t, "terraform", "tampered"), zipName) },
			wantErr: "checksum mismatch",
		},
		{
			name:    "no line for this os/arch",
			archive: makeZip(t, "terraform", "real"),
			sums:    func([]byte) string { return "abc  terraform_1.9.8_darwin_arm64.zip\n" },
			wantErr: "no checksum for",
		},
		{
			name:    "binary missing from archive",
			archive: makeZip(t, "README", "not a binary"),
			sums:    func(a []byte) string { return sumsLine(a, zipName) },
			wantErr: "not found in archive",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			baseURL := releaseServer(t, version, osName, arch, tc.archive, tc.sums(tc.archive))
			_, err := Download(context.Background(), baseURL, version, osName, arch, t.TempDir())
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}
