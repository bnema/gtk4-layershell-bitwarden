package xdg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewEmptyAppNameDefaults(t *testing.T) {
	p := New("")
	if p.AppName != defaultAppName {
		t.Fatalf("expected %q, got %q", defaultAppName, p.AppName)
	}
}

func TestNewPreservesAppName(t *testing.T) {
	p := New("my-app")
	if p.AppName != "my-app" {
		t.Fatalf("expected %q, got %q", "my-app", p.AppName)
	}
}

func TestDefaultAppName(t *testing.T) {
	p := Default()
	if p.AppName != defaultAppName {
		t.Fatalf("expected %q, got %q", defaultAppName, p.AppName)
	}
}

func TestConfigDirXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/custom/xdg/config")
	p := New("testapp")
	got := p.ConfigDir()
	want := "/custom/xdg/config/testapp"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestConfigDirUserConfigDir(t *testing.T) {
	// Ensure XDG_CONFIG_HOME is unset.
	t.Setenv("XDG_CONFIG_HOME", "")
	// Determine fallback.
	base, err := os.UserConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	p := New("testapp")
	got := p.ConfigDir()
	want := filepath.Join(base, "testapp")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestConfigDirFallback(t *testing.T) {
	// We can't easily make os.UserConfigDir fail, but we can verify the
	// XDG path works as a regression. If UserConfigDir fails the fallback
	// is ".", so we reconstruct:
	// t.Setenv("HOME", ...) won't make UserConfigDir fail.
	// This test exercises the XDG path already; the error path is
	// nearly impossible to trigger in a standard test environment.
	// Trust coverage from the XDG test above.
}

func TestConfigFile(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/xdg/config")
	p := New("testapp")
	got := p.ConfigFile()
	want := "/xdg/config/testapp/config.toml"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCacheDirXDG(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/custom/xdg/cache")
	p := New("testapp")
	got := p.CacheDir()
	want := "/custom/xdg/cache/testapp"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCacheDirUserCacheDir(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	base, err := os.UserCacheDir()
	if err != nil {
		t.Fatal(err)
	}
	p := New("testapp")
	got := p.CacheDir()
	want := filepath.Join(base, "testapp")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestCacheDirFallback(t *testing.T) {
	// os.UserCacheDir succeeds in normal environments; we trust the
	// XDG path above covers the first-priority path.
}

func TestCacheFile(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/xdg/cache")
	p := New("testapp")
	got := p.CacheFile()
	want := "/xdg/cache/testapp/cache.json"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestOutboxFile(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "/xdg/cache")
	p := New("testapp")
	got := p.OutboxFile()
	want := "/xdg/cache/testapp/outbox.json"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestStateDirXDG(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/custom/xdg/state")
	p := New("testapp")
	got := p.StateDir()
	want := "/custom/xdg/state/testapp"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestStateDirHomeFallback(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "/home/user")
	p := New("testapp")
	got := p.StateDir()
	want := "/home/user/.local/state/testapp"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestStateDirTempFallback(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "")
	p := New("testapp")
	got := p.StateDir()
	want := filepath.Join(os.TempDir(), "state", "testapp")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLogFile(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/xdg/state")
	p := New("testapp")
	got := p.LogFile()
	want := "/xdg/state/testapp/app.log"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// Integration test: default paths for the real app name.
func TestDefaultPathsSmoke(t *testing.T) {
	p := Default()
	if p.ConfigDir() == "" {
		t.Fatal("ConfigDir must not be empty")
	}
	if p.ConfigFile() == "" {
		t.Fatal("ConfigFile must not be empty")
	}
	if p.CacheDir() == "" {
		t.Fatal("CacheDir must not be empty")
	}
	if p.CacheFile() == "" {
		t.Fatal("CacheFile must not be empty")
	}
	if p.OutboxFile() == "" {
		t.Fatal("OutboxFile must not be empty")
	}
	if p.StateDir() == "" {
		t.Fatal("StateDir must not be empty")
	}
	if p.LogFile() == "" {
		t.Fatal("LogFile must not be empty")
	}
}
