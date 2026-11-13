package terraform

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestPlatform(t *testing.T) {
	tests := []struct {
		name             string
		osName, arch     string
		goos, goarch     string
		wantOS, wantArch string
	}{
		{name: "detects linux arm64", goos: "linux", goarch: "arm64", wantOS: "linux", wantArch: "arm64"},
		{name: "detects darwin arm64", goos: "darwin", goarch: "arm64", wantOS: "darwin", wantArch: "arm64"},
		{name: "flags override the running platform", osName: "linux", arch: "amd64", goos: "darwin", goarch: "arm64", wantOS: "linux", wantArch: "amd64"},
		{name: "one flag overrides only its half", arch: "amd64", goos: "darwin", goarch: "arm64", wantOS: "darwin", wantArch: "amd64"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotOS, gotArch, err := Platform(tc.osName, tc.arch, tc.goos, tc.goarch)
			assert.NilError(t, err)
			assert.Equal(t, gotOS, tc.wantOS)
			assert.Equal(t, gotArch, tc.wantArch)
		})
	}
}

func TestPlatformUnsupported(t *testing.T) {
	tests := []struct {
		name         string
		osName, arch string
		goos, goarch string
		want         string
	}{
		{name: "unsupported running os", goos: "windows", goarch: "amd64", want: `--os must be one of linux, darwin, not "windows"`},
		{name: "unsupported running arch", goos: "linux", goarch: "s390x", want: `--arch must be one of amd64, arm64, not "s390x"`},
		{name: "unsupported os flag", osName: "freebsd", goos: "linux", goarch: "amd64", want: `--os must be one of`},
		{name: "unsupported arch flag", arch: "riscv", goos: "linux", goarch: "amd64", want: `--arch must be one of`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := Platform(tc.osName, tc.arch, tc.goos, tc.goarch)
			assert.ErrorContains(t, err, tc.want)
		})
	}
}
