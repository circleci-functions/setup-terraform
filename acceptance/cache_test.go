//go:build acceptance

package setupterraform_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/circleci-functions/toolkit/testing/runner"

	"gotest.tools/v3/assert"

	"github.com/circleci-functions/setup-terraform/plugincache"
)

// mainTF needs one small provider, so terraform init has something to cache.
const mainTF = `terraform {
  required_providers {
    null = {
      source  = "hashicorp/null"
      version = "3.2.3"
    }
  }
}
`

// TestPluginCacheRoundTrip runs two jobs against one cache: the first downloads
// a provider and saves the plugin cache at the end of the job, and the second,
// starting from an empty plugin cache and no .terraform, gets the provider from
// the restored cache instead.
func TestPluginCacheRoundTrip(t *testing.T) {
	s, env := sandbox(t)
	project := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(project, "main.tf"), []byte(mainTF), 0o600))

	setup := func() {
		t.Helper()
		r := runner.New(t, binaryPath, runner.WithSandboxEnv(env))
		code, err := r.Run("--version", "1.9.8", "--checksum", "lock-a")
		out := r.StdoutString() + r.StderrString()
		assert.NilError(t, err, out)
		assert.Equal(t, code, 0, out)
	}
	initStep := "cd '" + project + "' && terraform init -input=false"

	setup()
	assert.Equal(t, len(s.Cache.PendingCacheSaves()), 1, "the plugin cache save should wait for the end of the job")
	out := laterStep(t, env, initStep)
	assert.Assert(t, strings.Contains(out, "Installing hashicorp/null"), "first init output: %s", out)
	s.Cache.EndJob(t)

	// The next job starts on a fresh executor: no plugins, no .terraform. The
	// lock file the first init wrote stays, as it would be committed.
	assert.NilError(t, os.RemoveAll(filepath.Join(s.Home, ".terraform.d")))
	assert.NilError(t, os.RemoveAll(filepath.Join(project, ".terraform")))

	setup()
	out = laterStep(t, env, initStep)
	assert.Assert(t, strings.Contains(out, "from the shared cache directory"), "second init output: %s", out)
}

// TestCacheDisabled checks --cache=false configures no plugin cache and
// registers no save.
func TestCacheDisabled(t *testing.T) {
	s, env := sandbox(t)
	r := runner.New(t, binaryPath, runner.WithSandboxEnv(env))
	code, err := r.Run("--version", "1.9.8", "--cache=false")
	out := r.StdoutString() + r.StderrString()
	assert.NilError(t, err, out)
	assert.Equal(t, code, 0, out)

	assert.Equal(t, len(s.Cache.PendingCacheSaves()), 0)
	_, err = os.Stat(filepath.Join(s.Home, ".terraform.d", plugincache.DropInName))
	assert.Assert(t, os.IsNotExist(err), "no plugin cache should be configured")
}
