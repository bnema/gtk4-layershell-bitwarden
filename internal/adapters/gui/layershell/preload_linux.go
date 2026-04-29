//go:build linux && !nogtk

package layershell

import (
	"os"
	"strings"
	"syscall"
)

// EnsurePreloaded re-execs the current process with gtk4-layer-shell in
// LD_PRELOAD when running on Wayland. gtk4-layer-shell installs a GDK backend
// hook that must be present before GTK initializes; loading the library later
// with dlopen can make IsSupported report false on compositors such as Niri.
func EnsurePreloaded() {
	if os.Getenv("WAYLAND_DISPLAY") == "" {
		return
	}
	if strings.Contains(os.Getenv("LD_PRELOAD"), "libgtk4-layer-shell") {
		return
	}

	libPath := findPreloadLibrary()
	if libPath == "" {
		return
	}

	preload := os.Getenv("LD_PRELOAD")
	if preload != "" {
		preload += ":"
	}
	preload += libPath

	exe, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return
	}

	env := os.Environ()
	found := false
	for i, entry := range env {
		if strings.HasPrefix(entry, "LD_PRELOAD=") {
			env[i] = "LD_PRELOAD=" + preload
			found = true
			break
		}
	}
	if !found {
		env = append(env, "LD_PRELOAD="+preload)
	}

	_ = syscall.Exec(exe, os.Args, env)
}

func findPreloadLibrary() string {
	for _, path := range []string{
		"/usr/lib/libgtk4-layer-shell.so.0",
		"/usr/lib64/libgtk4-layer-shell.so.0",
		"/usr/lib/x86_64-linux-gnu/libgtk4-layer-shell.so.0",
		"/usr/lib/aarch64-linux-gnu/libgtk4-layer-shell.so.0",
		"/usr/local/lib/libgtk4-layer-shell.so.0",
		"/usr/local/lib64/libgtk4-layer-shell.so.0",
		"/app/lib/libgtk4-layer-shell.so.0",
	} {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
