//go:build acceptance

package setupterraform_test

import (
	"os/exec"
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
		wantVersion string
	}{
		{"pinned exact version", "1.9.8", "Terraform v1.9.8"},
		{"prefix resolves to newest match", "1.9", "Terraform v1.9."},
		{"latest", "latest", "Terraform v"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, env := sandbox(t)

			r := runner.New(t, binaryPath, runner.WithSandboxEnv(env))
			code, err := r.Run("--version", tc.version, "--os", runtime.GOOS, "--arch", runtime.GOARCH)
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
