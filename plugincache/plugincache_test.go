package plugincache

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/circleci-functions/toolkit/sdk/sdktest"
	"gotest.tools/v3/assert"
)

// env returns an Env over a temp home, with vars standing in for the job's
// environment and exported recording what reaches $BASH_ENV.
func env(t *testing.T, goos string, vars map[string]string) (Env, map[string]string) {
	t.Helper()
	exported := map[string]string{}
	return Env{
		Getenv: func(k string) string { return vars[k] },
		GOOS:   goos,
		Home:   t.TempDir(),
		ExportEnv: func(v map[string]string) error {
			for k, val := range v {
				exported[k] = val
			}
			return nil
		},
	}, exported
}

func TestConfigureWritesDropIn(t *testing.T) {
	e, exported := env(t, "linux", map[string]string{})

	s, err := Configure(e)
	assert.NilError(t, err)

	want := filepath.Join(e.Home, ".terraform.d", "plugin-cache")
	assert.Equal(t, s.Dir, want)
	assert.Assert(t, isDir(s.Dir), "plugin cache directory should exist")

	got, err := os.ReadFile(filepath.Join(e.Home, ".terraform.d", DropInName))
	assert.NilError(t, err)
	assert.Assert(t, hclPluginCacheDir.Match(got), "drop-in sets no plugin_cache_dir:\n%s", got)
	assert.Equal(t, string(hclPluginCacheDir.FindSubmatch(got)[1]), filepath.ToSlash(want))
	assert.Equal(t, len(exported), 0)
}

func TestConfigureWindowsUsesAppData(t *testing.T) {
	appData := t.TempDir()
	e, _ := env(t, "windows", map[string]string{"APPDATA": appData})

	s, err := Configure(e)
	assert.NilError(t, err)
	assert.Equal(t, s.Dir, filepath.Join(appData, "terraform.d", "plugin-cache"))
	_, err = os.Stat(filepath.Join(appData, "terraform.d", DropInName))
	assert.NilError(t, err)
}

func TestConfigureUsesTFPluginCacheDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "plugins")
	e, exported := env(t, "linux", map[string]string{"TF_PLUGIN_CACHE_DIR": dir})

	s, err := Configure(e)
	assert.NilError(t, err)
	assert.Equal(t, s.Dir, dir)
	assert.Assert(t, isDir(dir))
	assertNoDropIn(t, e)
	assert.Equal(t, len(exported), 0)
}

func TestConfigureUsesConfiguredDir(t *testing.T) {
	tests := []struct {
		name    string
		file    string
		content string
	}{
		{"hcl", ".terraformrc", "disable_checkpoint = true\n  plugin_cache_dir = \"$HOME/plugins\"\n"},
		{"json", "cli.tfrc.json", `{"plugin_cache_dir": "$HOME/plugins"}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e, _ := env(t, "linux", nil)
			vars := map[string]string{}
			e.Getenv = func(k string) string { return vars[k] }
			vars["HOME"] = e.Home
			path := filepath.Join(e.Home, tc.file)
			if tc.file != ".terraformrc" {
				vars["TF_CLI_CONFIG_FILE"] = path
			}
			assert.NilError(t, os.WriteFile(path, []byte(tc.content), 0o600))

			s, err := Configure(e)
			assert.NilError(t, err)
			assert.Equal(t, s.Dir, filepath.Join(e.Home, "plugins"))
			assertNoDropIn(t, e)

			after, err := os.ReadFile(path)
			assert.NilError(t, err)
			assert.Equal(t, string(after), tc.content, "the job's own config file must not change")
		})
	}
}

func TestConfigureExportsWhenCLIConfigFileIsSet(t *testing.T) {
	e, exported := env(t, "linux", nil)
	path := filepath.Join(e.Home, "cli.tfrc")
	assert.NilError(t, os.WriteFile(path, []byte("disable_checkpoint = true\n"), 0o600))
	e.Getenv = func(k string) string {
		if k == "TF_CLI_CONFIG_FILE" {
			return path
		}
		return ""
	}

	s, err := Configure(e)
	assert.NilError(t, err)
	assert.Equal(t, exported["TF_PLUGIN_CACHE_DIR"], s.Dir)
	assertNoDropIn(t, e)
}

func TestKeys(t *testing.T) {
	k := Key{Key: "infra", Checksum: "abc", Platform: "linux-amd64"}
	now := time.Unix(1_700_000_000, 0)
	window := now.Unix() / int64(cacheRotation.Seconds()) * int64(cacheRotation.Seconds())

	prefix := "setup-terraform-plugins-v1-infra-linux-amd64-" + itoa(window) + "-"
	assert.Equal(t, k.saveKey(now), prefix+"abc")
	assert.DeepEqual(t, k.restoreKeys(now), []string{prefix + "abc", prefix})

	next := now.Add(cacheRotation)
	assert.Assert(t, k.saveKey(next) != k.saveKey(now), "the key should rotate")
}

func TestSaveAtTeardownRoundTrips(t *testing.T) {
	srv := sdktest.NewServer(t)
	client := srv.Client()
	ctx := context.Background()
	k := Key{Checksum: "lock-a", Platform: "linux-amd64"}

	dir := t.TempDir()
	assert.NilError(t, os.WriteFile(filepath.Join(dir, "marker"), []byte("hit"), 0o600))

	Save(ctx, client.AtTeardown(), k, dir)
	assert.Equal(t, len(srv.PendingCacheSaves()), 1, "the save should wait for the end of the job")
	srv.EndJob(t)
	assert.NilError(t, os.RemoveAll(dir))

	// A new checksum still restores from the same window.
	Restore(ctx, client, Key{Checksum: "lock-b", Platform: "linux-amd64"})
	got, err := os.ReadFile(filepath.Join(dir, "marker")) //nolint:gosec // test temp file
	assert.NilError(t, err)
	assert.Equal(t, string(got), "hit")
}

func assertNoDropIn(t *testing.T, e Env) {
	t.Helper()
	_, err := os.Stat(filepath.Join(configDir(e), DropInName))
	assert.Assert(t, os.IsNotExist(err), "no drop-in should be written")
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
