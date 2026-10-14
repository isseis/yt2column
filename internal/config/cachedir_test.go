//go:build test

package config

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestDefaultCacheDir(t *testing.T) {
	cases := []struct {
		name   string
		goos   string
		env    map[string]string
		want   string
		wantOK bool
	}{
		{
			name:   "darwin home",
			goos:   "darwin",
			env:    map[string]string{"HOME": "/Users/u"},
			want:   filepath.Join("/Users/u", "Library", "Caches", "yt2column"),
			wantOK: true,
		},
		{
			name:   "darwin ignores xdg",
			goos:   "darwin",
			env:    map[string]string{"HOME": "/Users/u", "XDG_CACHE_HOME": "/xdg"},
			want:   filepath.Join("/Users/u", "Library", "Caches", "yt2column"),
			wantOK: true,
		},
		{name: "darwin home unset", goos: "darwin", env: map[string]string{}, wantOK: false},
		{name: "darwin home empty", goos: "darwin", env: map[string]string{"HOME": ""}, wantOK: false},
		{name: "darwin home relative", goos: "darwin", env: map[string]string{"HOME": "rel"}, wantOK: false},
		{
			name:   "linux xdg",
			goos:   "linux",
			env:    map[string]string{"XDG_CACHE_HOME": "/xdg"},
			want:   filepath.Join("/xdg", "yt2column"),
			wantOK: true,
		},
		{
			name:   "linux xdg wins over home",
			goos:   "linux",
			env:    map[string]string{"XDG_CACHE_HOME": "/xdg", "HOME": "/home/u"},
			want:   filepath.Join("/xdg", "yt2column"),
			wantOK: true,
		},
		{
			name:   "linux empty xdg falls back to home",
			goos:   "linux",
			env:    map[string]string{"XDG_CACHE_HOME": "", "HOME": "/home/u"},
			want:   filepath.Join("/home/u", ".cache", "yt2column"),
			wantOK: true,
		},
		{
			name:   "linux home",
			goos:   "linux",
			env:    map[string]string{"HOME": "/home/u"},
			want:   filepath.Join("/home/u", ".cache", "yt2column"),
			wantOK: true,
		},
		{
			name:   "linux relative xdg does not fall back",
			goos:   "linux",
			env:    map[string]string{"XDG_CACHE_HOME": "rel", "HOME": "/home/u"},
			wantOK: false,
		},
		{name: "linux empty home", goos: "linux", env: map[string]string{"HOME": ""}, wantOK: false},
		{name: "linux relative home", goos: "linux", env: map[string]string{"HOME": "rel"}, wantOK: false},
		{name: "linux no home", goos: "linux", env: map[string]string{}, wantOK: false},
		{name: "windows", goos: "windows", env: map[string]string{"HOME": "/u"}, wantOK: false},
		{name: "plan9", goos: "plan9", env: map[string]string{"HOME": "/u"}, wantOK: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := defaultCacheDir(tc.goos, envLookup(tc.env))
			if ok != tc.wantOK {
				t.Fatalf("defaultCacheDir(%q, ...) ok = %v, want %v", tc.goos, ok, tc.wantOK)
			}
			if got != tc.want {
				t.Errorf("defaultCacheDir(%q, ...) = %q, want %q", tc.goos, got, tc.want)
			}
		})
	}
}

// TestDefaultConfigDir fixes the configuration directory table: the OS branch,
// the XDG variable, and, when the directory cannot be derived, the variable
// that prevented it and the sentinel for the rejection.
func TestDefaultConfigDir(t *testing.T) {
	cases := []struct {
		name    string
		goos    string
		env     map[string]string
		want    string
		wantVar string // non-empty when derivation must fail
		wantErr error
	}{
		{
			name: "darwin home",
			goos: "darwin",
			env:  map[string]string{"HOME": "/Users/u"},
			want: filepath.Join("/Users/u", "Library", "Application Support", "yt2column"),
		},
		{
			name: "darwin ignores xdg",
			goos: "darwin",
			env:  map[string]string{"HOME": "/Users/u", "XDG_CONFIG_HOME": "/xdg"},
			want: filepath.Join("/Users/u", "Library", "Application Support", "yt2column"),
		},
		{name: "darwin home unset", goos: "darwin", env: map[string]string{}, wantVar: "HOME", wantErr: ErrMissing},
		{name: "darwin home empty", goos: "darwin", env: map[string]string{"HOME": ""}, wantVar: "HOME", wantErr: ErrMissing},
		{name: "darwin home relative", goos: "darwin", env: map[string]string{"HOME": "rel"}, wantVar: "HOME", wantErr: ErrMissing},
		{
			name: "linux xdg",
			goos: "linux",
			env:  map[string]string{"XDG_CONFIG_HOME": "/xdg"},
			want: filepath.Join("/xdg", "yt2column"),
		},
		{
			name: "linux xdg wins over home",
			goos: "linux",
			env:  map[string]string{"XDG_CONFIG_HOME": "/xdg", "HOME": "/home/u"},
			want: filepath.Join("/xdg", "yt2column"),
		},
		{
			name: "linux empty xdg falls back to home",
			goos: "linux",
			env:  map[string]string{"XDG_CONFIG_HOME": "", "HOME": "/home/u"},
			want: filepath.Join("/home/u", ".config", "yt2column"),
		},
		{
			name: "linux home",
			goos: "linux",
			env:  map[string]string{"HOME": "/home/u"},
			want: filepath.Join("/home/u", ".config", "yt2column"),
		},
		{
			name:    "linux relative xdg does not fall back",
			goos:    "linux",
			env:     map[string]string{"XDG_CONFIG_HOME": "rel", "HOME": "/home/u"},
			wantVar: "XDG_CONFIG_HOME",
			wantErr: ErrInvalid,
		},
		{name: "linux no home", goos: "linux", env: map[string]string{}, wantVar: "HOME", wantErr: ErrMissing},
		{name: "linux empty home", goos: "linux", env: map[string]string{"HOME": ""}, wantVar: "HOME", wantErr: ErrMissing},
		{name: "linux relative home", goos: "linux", env: map[string]string{"HOME": "rel"}, wantVar: "HOME", wantErr: ErrMissing},
		{name: "windows", goos: "windows", env: map[string]string{"HOME": "/u"}, wantVar: "HOME", wantErr: ErrMissing},
		{name: "plan9", goos: "plan9", env: map[string]string{"HOME": "/u"}, wantVar: "HOME", wantErr: ErrMissing},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := defaultConfigDir(tc.goos, envLookup(tc.env))
			if tc.wantVar == "" {
				if err != nil {
					t.Fatalf("defaultConfigDir(%q, ...) error = %v, want nil", tc.goos, err)
				}
				if got != tc.want {
					t.Errorf("defaultConfigDir(%q, ...) = %q, want %q", tc.goos, got, tc.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("defaultConfigDir(%q, ...) = %q, want an error naming %s", tc.goos, got, tc.wantVar)
			}
			if err.Name != tc.wantVar {
				t.Errorf("defaultConfigDir(%q, ...) error names %q, want %q", tc.goos, err.Name, tc.wantVar)
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("defaultConfigDir(%q, ...) error = %v, want it to wrap %v", tc.goos, err, tc.wantErr)
			}
		})
	}
}
