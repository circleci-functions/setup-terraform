// Package plugincache points Terraform at a provider plugin cache directory for
// the rest of the job, and keeps that directory in the CircleCI cache across
// jobs.
//
// A step inherits its environment from the job, so exporting
// TF_PLUGIN_CACHE_DIR would reach only steps that source $BASH_ENV. Terraform
// also reads every *.tfrc file in its configuration directory, as long as
// TF_CLI_CONFIG_FILE is unset, so writing one there reaches every later step
// whatever shell it runs in, without touching a CLI configuration file the job
// already has.
package plugincache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/circleci-functions/toolkit/sdk"
)

// DropInName is the file written into Terraform's configuration directory.
const DropInName = "setup-terraform.tfrc"

// cacheRotation is how often the cache key rotates. Terraform never removes a
// provider version from its plugin cache, so starting afresh is the only thing
// that keeps the cache from growing without limit.
const cacheRotation = 72 * time.Hour

// Env is what Configure reads from and writes to, so tests can stand in for
// the job.
type Env struct {
	Getenv func(string) string
	GOOS   string
	Home   string
	// ExportEnv passes variables to later steps through $BASH_ENV.
	ExportEnv func(map[string]string) error
}

// Setup is where the plugin cache is, and how Terraform was told about it.
type Setup struct {
	Dir string
	How string
}

// Configure picks the plugin cache directory and makes sure Terraform in later
// steps uses it, in order of preference:
//
//  1. TF_PLUGIN_CACHE_DIR, when the job sets it.
//  2. plugin_cache_dir in the CLI configuration file Terraform reads, when it
//     sets one.
//  3. Otherwise plugin-cache in Terraform's configuration directory, named in
//     a DropInName file there. With TF_CLI_CONFIG_FILE set, Terraform skips
//     that directory, so the location is exported as TF_PLUGIN_CACHE_DIR
//     through $BASH_ENV instead.
//
// It never edits a file it did not write. The directory is created, since
// Terraform ignores a plugin cache that doesn't exist.
func Configure(e Env) (Setup, error) {
	s, err := locate(e)
	if err != nil {
		return Setup{}, err
	}
	if err := os.MkdirAll(s.Dir, 0o750); err != nil {
		return Setup{}, fmt.Errorf("create plugin cache directory: %w", err)
	}
	return s, nil
}

func locate(e Env) (Setup, error) {
	if dir := e.Getenv("TF_PLUGIN_CACHE_DIR"); dir != "" {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return Setup{}, err
		}
		return Setup{Dir: abs, How: "from TF_PLUGIN_CACHE_DIR"}, nil
	}

	configFile := e.Getenv("TF_CLI_CONFIG_FILE")
	explicit := configFile != ""
	if !explicit {
		configFile = defaultConfigFile(e)
	}
	dir, ok, err := configuredDir(configFile, e.Getenv)
	if err != nil {
		return Setup{}, err
	}
	if ok {
		return Setup{Dir: dir, How: "from plugin_cache_dir in " + configFile}, nil
	}

	confDir := configDir(e)
	dir = filepath.Join(confDir, "plugin-cache")
	if explicit {
		if err := e.ExportEnv(map[string]string{"TF_PLUGIN_CACHE_DIR": dir}); err != nil {
			return Setup{}, fmt.Errorf("export TF_PLUGIN_CACHE_DIR: %w", err)
		}
		return Setup{Dir: dir, How: "exported as TF_PLUGIN_CACHE_DIR to $BASH_ENV, since TF_CLI_CONFIG_FILE is set; only bash steps see it"}, nil
	}

	dropIn := filepath.Join(confDir, DropInName)
	if err := os.MkdirAll(confDir, 0o750); err != nil {
		return Setup{}, err
	}
	content := "# Written by setup-terraform, so Terraform caches provider plugins.\n" +
		"plugin_cache_dir = " + strconv.Quote(filepath.ToSlash(dir)) + "\n"
	if err := os.WriteFile(dropIn, []byte(content), 0o600); err != nil {
		return Setup{}, fmt.Errorf("write %s: %w", dropIn, err)
	}
	return Setup{Dir: dir, How: "named in " + dropIn}, nil
}

// configDir is the directory Terraform reads *.tfrc files and plugins from.
func configDir(e Env) string {
	if e.GOOS == "windows" {
		return filepath.Join(e.Getenv("APPDATA"), "terraform.d")
	}
	return filepath.Join(e.Home, ".terraform.d")
}

// defaultConfigFile is the CLI configuration file Terraform reads when
// TF_CLI_CONFIG_FILE is unset.
func defaultConfigFile(e Env) string {
	if e.GOOS == "windows" {
		return filepath.Join(e.Getenv("APPDATA"), "terraform.rc")
	}
	return filepath.Join(e.Home, ".terraformrc")
}

var hclPluginCacheDir = regexp.MustCompile(`(?m)^\s*plugin_cache_dir\s*=\s*"([^"]*)"`)

// configuredDir returns the plugin_cache_dir a CLI configuration file sets,
// with environment variables expanded as Terraform expands them. A missing
// file sets nothing.
func configuredDir(path string, getenv func(string) string) (string, bool, error) {
	data, err := os.ReadFile(path) //nolint:gosec // the CLI configuration file Terraform itself reads
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}

	var dir string
	if strings.HasSuffix(path, ".json") {
		var conf struct {
			PluginCacheDir string `json:"plugin_cache_dir"`
		}
		if err := json.Unmarshal(data, &conf); err != nil {
			return "", false, fmt.Errorf("parse %s: %w", path, err)
		}
		dir = conf.PluginCacheDir
	} else if m := hclPluginCacheDir.FindSubmatch(data); m != nil {
		dir = string(m[1])
	}
	if dir == "" {
		return "", false, nil
	}
	return os.Expand(dir, getenv), true, nil
}

// Saver saves paths under a cache key. Both *sdk.Client and the *sdk.Teardown
// returned by Client.AtTeardown satisfy it.
type Saver interface {
	SaveCache(ctx context.Context, key string, paths []string, opts ...sdk.CacheOption) error
}

// Key names the cache entries for one job's plugin cache.
type Key struct {
	// Key partitions the cache, for example per root module in a monorepo.
	Key string
	// Checksum, usually of .terraform.lock.hcl, starts a fresh entry when it
	// changes.
	Checksum string
	// Platform is the os-arch Terraform was installed for, since providers
	// are built per platform.
	Platform string
}

// Restore restores the plugin cache, preferring an entry saved with the same
// checksum and falling back to any entry from the current rotation window.
// Cache errors are logged, never returned.
func Restore(ctx context.Context, client *sdk.Client, k Key) {
	_, err := client.RestoreCache(ctx, k.restoreKeys(time.Now()),
		sdk.WithStepName("Restoring Terraform provider plugin cache"))
	sdk.LogCacheErr("restore plugins", err)
}

// Save saves dir as the plugin cache. Pass client.AtTeardown() to save once
// the job is done, after the steps that run terraform init.
func Save(ctx context.Context, saver Saver, k Key, dir string) {
	err := saver.SaveCache(ctx, k.saveKey(time.Now()), []string{dir},
		sdk.WithStepName("Saving Terraform provider plugin cache"))
	sdk.LogCacheErr("save plugins", err)
}

// saveKey is the key the plugin cache saves under:
//
//	setup-terraform-plugins-v1-<key>-<os>-<arch>-<rotating epoch>-<checksum>
//
// The epoch comes before the checksum so that a prefix broad enough to match
// any checksum still stops at the current window.
func (k Key) saveKey(now time.Time) string {
	return k.windowPrefix(now) + k.Checksum
}

// restoreKeys are the keys a restore tries, in order: the exact key, then any
// entry from the same window. Neither reaches an earlier window, which is what
// keeps the cache bounded.
func (k Key) restoreKeys(now time.Time) []string {
	return []string{k.windowPrefix(now) + k.Checksum, k.windowPrefix(now)}
}

// windowPrefix matches every entry saved in now's rotation window. The epoch
// is a fixed-width unix timestamp followed by a separator, so it can't match a
// neighbouring window.
func (k Key) windowPrefix(now time.Time) string {
	secs := int64(cacheRotation.Seconds())
	epoch := now.Unix() / secs * secs
	return fmt.Sprintf("setup-terraform-plugins-v1-%s-%s-%d-", k.Key, k.Platform, epoch)
}
