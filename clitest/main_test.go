// Package setupterraform_clitest exercises the compiled binary's CLI contract
// hermetically: flag parsing, help, and enum validation, none of which touch
// the network. Real installs are covered by acceptance/.
package setupterraform_clitest

import (
	"context"
	"fmt"
	"os"
	"testing"

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
