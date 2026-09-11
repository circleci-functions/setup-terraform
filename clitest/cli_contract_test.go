package setupterraform_clitest

import (
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
	for _, want := range []string{"Install the Terraform CLI", "--version", "--os", "--arch"} {
		assert.Assert(t, strings.Contains(out, want), "help missing %q:\n%s", want, out)
	}
}

func TestFlagErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"invalid os", []string{"--os", "freebsd"}, "--os"},
		{"invalid arch", []string{"--arch", "riscv"}, "--arch"},
		{"unknown flag", []string{"--nope"}, "unknown flag"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, out := run(t, tc.args...)
			assert.Assert(t, code != 0, "expected non-zero exit, got 0:\n%s", out)
			assert.Assert(t, strings.Contains(out, tc.want), "output missing %q:\n%s", tc.want, out)
		})
	}
}
