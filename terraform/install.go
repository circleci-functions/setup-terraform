package terraform

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// binaryName is the executable PATH resolves, and the name pathlink links.
const binaryName = "terraform"

// exeName is the executable in the release archive for osName.
func exeName(osName string) string {
	if osName == "windows" {
		return binaryName + ".exe"
	}
	return binaryName
}

// errNotFound marks a download the release server has no file for.
var errNotFound = errors.New("not found")

// Download fetches the Terraform release archive for version/osName/arch from
// baseURL, verifies it against the release's published SHA256SUMS, extracts the
// terraform binary into destDir, and returns the path to that binary.
func Download(ctx context.Context, baseURL, version, osName, arch, destDir string) (string, error) {
	zipName := fmt.Sprintf("terraform_%s_%s_%s.zip", version, osName, arch)
	prefix := baseURL + "/terraform/" + version + "/"

	archive, err := get(ctx, prefix+zipName)
	if errors.Is(err, errNotFound) {
		return "", fmt.Errorf("terraform %s has no %s_%s build: %w", version, osName, arch, err)
	}
	if err != nil {
		return "", err
	}
	sums, err := get(ctx, prefix+fmt.Sprintf("terraform_%s_SHA256SUMS", version))
	if err != nil {
		return "", err
	}

	expected, err := sumFor(sums, zipName)
	if err != nil {
		return "", err
	}
	got := sha256.Sum256(archive)
	if hex.EncodeToString(got[:]) != expected {
		return "", fmt.Errorf("checksum mismatch for %s: expected %s, got %x", zipName, expected, got)
	}

	return extractBinary(archive, exeName(osName), destDir)
}

// get returns the body of a successful GET, or an error for a transport failure
// or non-200 status.
func get(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("download %s: %w", url, errNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// sumFor returns the checksum recorded for filename in a SHA256SUMS file, whose
// lines are "<hex-sha>  <filename>".
func sumFor(sums []byte, filename string) (string, error) {
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == filename {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("no checksum for %s in SHA256SUMS", filename)
}

// extractBinary writes the entry called name from the zip archive into destDir
// and returns its path.
func extractBinary(archive []byte, name, destDir string) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return "", fmt.Errorf("open terraform archive: %w", err)
	}
	if err := os.MkdirAll(destDir, 0750); err != nil {
		return "", err
	}
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()

		dest := filepath.Join(destDir, name)
		//nolint:gosec // dest is destDir joined with a constant filename; the 0755 exec bit is required to run terraform
		out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return "", err
		}
		defer out.Close()
		if _, err := io.Copy(out, rc); err != nil { //nolint:gosec // release archive from a checksum-verified download
			return "", err
		}
		return dest, nil
	}
	return "", fmt.Errorf("%s not found in archive", name)
}
