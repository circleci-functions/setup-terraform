// Package terraform resolves and installs Terraform CLI releases from
// HashiCorp's release server.
package terraform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// DefaultBaseURL is HashiCorp's release server, the source of Terraform
// releases and the version index.
const DefaultBaseURL = "https://releases.hashicorp.com"

// stableVersion matches a fully-qualified stable version (1.9.8). It rejects
// prereleases such as 1.10.0-rc1, so those are never selected.
var stableVersion = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// ResolveVersion resolves spec to a concrete stable Terraform version.
//
// An exact version (1.9.8) is returned as-is without contacting the server.
// "latest" returns the newest stable release; any other value is treated as a
// prefix (1.9, 1) and returns the newest stable release whose version starts
// with it. Prereleases are never returned.
func ResolveVersion(ctx context.Context, baseURL, spec string) (string, error) {
	if stableVersion.MatchString(spec) {
		return spec, nil
	}

	versions, err := fetchVersions(ctx, baseURL)
	if err != nil {
		return "", err
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("no stable terraform versions found at %s", baseURL)
	}

	if spec == "latest" {
		return versions[0], nil
	}

	for _, v := range versions {
		if strings.HasPrefix(v, spec) {
			return v, nil
		}
	}
	return "", fmt.Errorf("no stable terraform version matches %q", spec)
}

// fetchVersions returns the stable versions from the release index, newest first.
func fetchVersions(ctx context.Context, baseURL string) ([]string, error) {
	url := baseURL + "/terraform/index.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch terraform version index: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch terraform version index: %s", resp.Status)
	}

	var index struct {
		Versions map[string]json.RawMessage `json:"versions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&index); err != nil {
		return nil, fmt.Errorf("decode terraform version index: %w", err)
	}

	versions := make([]string, 0, len(index.Versions))
	for v := range index.Versions {
		if stableVersion.MatchString(v) {
			versions = append(versions, v)
		}
	}
	sort.Slice(versions, func(i, j int) bool {
		return compareVersions(versions[i], versions[j]) > 0
	})
	return versions, nil
}

// compareVersions orders two N.N.N version strings numerically, returning a
// positive value when a is newer than b, negative when older, zero when equal.
func compareVersions(a, b string) int {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		ai, _ := strconv.Atoi(as[i])
		bi, _ := strconv.Atoi(bs[i])
		if ai != bi {
			return ai - bi
		}
	}
	return len(as) - len(bs)
}
