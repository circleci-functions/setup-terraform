//go:build acceptance

// Package setupterraform_test exercises setup-terraform against real
// infrastructure: it installs Terraform releases over the network and confirms
// a later step resolves terraform on PATH. Tagged out of `task test` because it
// needs network. The CLI contract itself is covered hermetically by clitest/.
package setupterraform_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/circleci-functions/toolkit/sdk/sdktest"
	"github.com/circleci-functions/toolkit/testing/compiler"
)

var binaryPath string

func TestMain(m *testing.M) {
	status, err := runTests(m)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	os.Exit(status)
}

func runTests(m *testing.M) (int, error) {
	p := compiler.NewParallel(1)
	defer p.Cleanup()

	p.Add(compiler.Work{
		Result: &binaryPath,
		Name:   "setup-terraform",
		Target: "..",
		Source: ".",
	})

	if err := p.Run(context.Background()); err != nil {
		return 0, err
	}
	return m.Run(), nil
}

// sandbox returns a sandbox plus its environment with the sandbox bin directory
// prepended to PATH, so the symlink setup-terraform creates lands in a temp
// directory and a later step resolving terraform stays contained to the test.
func sandbox(t *testing.T) (*sdktest.Sandbox, map[string]string) {
	t.Helper()
	s := sdktest.NewSandbox(t)
	env := s.Env()
	env["PATH"] = s.BinDir + string(os.PathListSeparator) + os.Getenv("PATH")
	return s, env
}

func envSlice(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}
