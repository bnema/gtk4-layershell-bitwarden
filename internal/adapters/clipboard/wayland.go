package clipboard

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"strings"
)

const wlCopyBinary = "wl-copy"

var ErrClipboardUnavailable = errors.New("clipboard: no clipboard backend available")

type wlCopyRunner func(text string, args ...string) error

type wlCopyClearRunner func(args ...string) error

type wlCopyCommandError struct {
	err    error
	stderr string
}

func (e *wlCopyCommandError) Error() string {
	if strings.TrimSpace(e.stderr) == "" {
		return e.err.Error()
	}
	return e.err.Error() + ": " + strings.TrimSpace(e.stderr)
}

func (e *wlCopyCommandError) Unwrap() error {
	return e.err
}

func NewWaylandPreferred(fallbackSet Setter, fallbackClear Clearer) *Adapter {
	var primarySet Setter
	var primaryClear Clearer
	if _, err := exec.LookPath(wlCopyBinary); err == nil {
		primarySet = wlCopySetter()
		primaryClear = wlCopyClearer()
	}
	return New(preferSetter(primarySet, fallbackSet), preferClearer(primaryClear, fallbackClear))
}

func preferSetter(primary, fallback Setter) Setter {
	return func(text string) error {
		if primary != nil {
			if err := primary(text); err == nil {
				return nil
			} else if fallback == nil {
				return err
			}
		}
		if fallback != nil {
			return fallback(text)
		}
		return ErrClipboardUnavailable
	}
}

func preferClearer(primary, fallback Clearer) Clearer {
	return func() error {
		if primary != nil {
			if err := primary(); err == nil {
				return nil
			} else if fallback == nil {
				return err
			}
		}
		if fallback != nil {
			return fallback()
		}
		return ErrClipboardUnavailable
	}
}

func wlCopySetter() Setter {
	return wlCopySetterWithRunner(runWlCopy)
}

func wlCopySetterWithRunner(run wlCopyRunner) Setter {
	return func(text string) error {
		err := run(text, "--trim-newline", "--sensitive")
		if err != nil && isUnsupportedSensitiveFlagError(err) {
			return run(text, "--trim-newline")
		}
		return err
	}
}

func wlCopyClearer() Clearer {
	return wlCopyClearerWithRunner(runWlCopyClear)
}

func wlCopyClearerWithRunner(run wlCopyClearRunner) Clearer {
	return func() error {
		return run("--clear")
	}
}

func runWlCopy(text string, args ...string) error {
	return runWlCopyCommand(strings.NewReader(text), args...)
}

func runWlCopyClear(args ...string) error {
	return runWlCopyCommand(nil, args...)
}

func runWlCopyCommand(stdin io.Reader, args ...string) error {
	cmd := exec.Command(wlCopyBinary, args...)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return &wlCopyCommandError{err: err, stderr: stderr.String()}
	}
	return nil
}

func isUnsupportedSensitiveFlagError(err error) bool {
	var runErr *wlCopyCommandError
	if !errors.As(err, &runErr) {
		return false
	}
	stderr := strings.ToLower(runErr.stderr)
	if !strings.Contains(stderr, "sensitive") {
		return false
	}
	for _, marker := range []string{"unknown option", "unrecognized option", "invalid option", "illegal option"} {
		if strings.Contains(stderr, marker) {
			return true
		}
	}
	return false
}
