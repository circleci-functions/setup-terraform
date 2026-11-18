package setupterraform_clitest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/circleci-functions/toolkit/sdk/sdktest"
	"github.com/circleci-functions/toolkit/testing/runner"
	"gotest.tools/v3/assert"
)

func run(t *testing.T, args ...string) (int, string) {
	t.Helper()
	s := sdktest.NewSandbox(t)
	r := runner.New(t, binaryPath, runner.WithSandboxEnv(s.Env()))
	code, _ := r.Run(args...)
	return code, r.StdoutString() + r.StderrString()
}

func TestHelp(t *testing.T) {
	code, out := run(t, "--help")
	assert.Equal(t, code, 0, out)
	for _, want := range []string{
		"Install the Terraform CLI",
		"--version",
		"--version-file",
		"--os",
		"--arch",
		"--cache",
		"--key",
		"--checksum",
		"--sha256-file",
	} {
		assert.Assert(t, strings.Contains(out, want), "help missing %q:\n%s", want, out)
	}
}

func emptyFile(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".terraform-version")
	assert.NilError(t, os.WriteFile(path, nil, 0o600))
	return path
}

func TestFlagErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "invalid os",
			args: []string{"--os", "freebsd"},
			want: "--os",
		},
		{
			name: "invalid arch",
			args: []string{"--arch", "riscv"},
			want: "--arch",
		},
		{
			name: "unknown flag",
			args: []string{"--nope"},
			want: "unknown flag",
		},
		{
			name: "version and version file",
			args: []string{"--version", "1.9.8", "--version-file", ".terraform-version"},
			want: "--version and --version-file can't be used together",
		},
		{
			name: "missing version file",
			args: []string{"--version-file", "does-not-exist/.terraform-version"},
			want: "read version file",
		},
		{
			name: "empty version file",
			args: []string{"--version-file", emptyFile(t)},
			want: "has no version in it",
		},
		{
			name: "sha256 file with a prefix",
			args: []string{"--version", "1.9", "--sha256-file", "SHA256SUMS"},
			want: "--sha256-file needs an exact version",
		},
		{
			name: "sha256 file with latest",
			args: []string{"--sha256-file", "SHA256SUMS"},
			want: "--sha256-file needs an exact version",
		},
		{
			name: "missing sha256 file",
			args: []string{"--version", "1.9.8", "--sha256-file", "does-not-exist/SHA256SUMS"},
			want: "read sha256 file",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, out := run(t, tc.args...)
			assert.Assert(t, code != 0, "expected non-zero exit, got 0:\n%s", out)
			assert.Assert(t, strings.Contains(out, tc.want), "output missing %q:\n%s", tc.want, out)
		})
	}
}
