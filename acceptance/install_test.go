//go:build acceptance

package setupterraform_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/circleci-functions/toolkit/testing/runner"
	"gotest.tools/v3/assert"
)

func TestInstall(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		versionFile string
		wantVersion string
	}{
		{name: "pinned exact version", version: "1.9.8", wantVersion: "Terraform v1.9.8"},
		{name: "prefix resolves to newest match", version: "1.9", wantVersion: "Terraform v1.9."},
		{name: "latest", version: "latest", wantVersion: "Terraform v"},
		{name: "no version installs latest", wantVersion: "Terraform v"},
		{name: "version file", versionFile: "1.9.8\n", wantVersion: "Terraform v1.9.8"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, env := sandbox(t)

			args := []string{"--os", runtime.GOOS, "--arch", runtime.GOARCH}
			if tc.version != "" {
				args = append(args, "--version", tc.version)
			}
			if tc.versionFile != "" {
				path := filepath.Join(t.TempDir(), ".terraform-version")
				assert.NilError(t, os.WriteFile(path, []byte(tc.versionFile), 0o600))
				args = append(args, "--version-file", path)
			}

			r := runner.New(t, binaryPath, runner.WithSandboxEnv(env))
			code, err := r.Run(args...)
			assert.NilError(t, err, r.StdoutString()+r.StderrString())
			assert.Equal(t, code, 0, r.StdoutString()+r.StderrString())

			out := laterStep(t, env, "terraform version")
			assert.Assert(t, strings.Contains(out, tc.wantVersion), "later step output: %s", out)
		})
	}
}

// laterStep runs script in a fresh shell with env, standing in for a subsequent
// CircleCI step that shares no shell with the install.
func laterStep(t *testing.T, env map[string]string, script string) string {
	t.Helper()
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = envSlice(env)
	out, err := cmd.CombinedOutput()
	assert.NilError(t, err, string(out))
	return string(out)
}
