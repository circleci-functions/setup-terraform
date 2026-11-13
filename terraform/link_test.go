package terraform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/circleci-functions/toolkit/sdk/sdktest"
	"gotest.tools/v3/assert"
)

// TestLinkOnPath runs only inside a CircleCI job: pathlink resolves against the
// real PATH and cannot be sandboxed, so LaterStep is the only way to prove the
// link carries to a step that shares no shell with the install.
func TestLinkOnPath(t *testing.T) {
	sdktest.SkipUnlessInJob(t)

	dir := t.TempDir()
	script := "#!/bin/sh\necho 'Terraform v9.9.9'\n"
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "terraform"), []byte(script), 0755))

	assert.NilError(t, linkOnPath(dir, "9.9.9"))

	out, err := sdktest.LaterStep(t, "terraform", "version")
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(out, "Terraform v9.9.9"), "later step output: %s", out)
}
