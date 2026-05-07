package clipboard

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPreferSetter_PrimarySuccessSkipsFallback(t *testing.T) {
	calls := make([]string, 0, 2)
	setter := preferSetter(
		func(text string) error {
			calls = append(calls, "primary:"+text)
			return nil
		},
		func(text string) error {
			calls = append(calls, "fallback:"+text)
			return nil
		},
	)

	require.NoError(t, setter("secret"))
	require.Equal(t, []string{"primary:secret"}, calls)
}

func TestPreferSetter_PrimaryErrorFallsBack(t *testing.T) {
	calls := make([]string, 0, 2)
	setter := preferSetter(
		func(text string) error {
			calls = append(calls, "primary:"+text)
			return errors.New("primary failed")
		},
		func(text string) error {
			calls = append(calls, "fallback:"+text)
			return nil
		},
	)

	require.NoError(t, setter("secret"))
	require.Equal(t, []string{"primary:secret", "fallback:secret"}, calls)
}

func TestPreferClearer_ReturnsPrimaryErrorWhenNoFallback(t *testing.T) {
	wantErr := errors.New("clear failed")
	clearer := preferClearer(
		func() error { return wantErr },
		nil,
	)

	err := clearer()
	require.ErrorIs(t, err, wantErr)
}

func TestPreferSetter_NoPrimaryNoFallbackReturnsUnavailable(t *testing.T) {
	setter := preferSetter(nil, nil)
	require.ErrorIs(t, setter("secret"), ErrClipboardUnavailable)
}

func TestPreferSetter_NilPrimaryDelegatesToFallback(t *testing.T) {
	calls := make([]string, 0, 1)
	setter := preferSetter(nil, func(text string) error {
		calls = append(calls, text)
		return nil
	})

	require.NoError(t, setter("secret"))
	require.Equal(t, []string{"secret"}, calls)
}

func TestPreferClearer_NoPrimaryNoFallbackReturnsUnavailable(t *testing.T) {
	clearer := preferClearer(nil, nil)
	require.ErrorIs(t, clearer(), ErrClipboardUnavailable)
}

func TestPreferClearer_NilPrimaryDelegatesToFallback(t *testing.T) {
	calls := make([]string, 0, 1)
	clearer := preferClearer(nil, func() error {
		calls = append(calls, "fallback")
		return nil
	})

	require.NoError(t, clearer())
	require.Equal(t, []string{"fallback"}, calls)
}

func TestPreferClearer_PrimaryErrorFallsBack(t *testing.T) {
	calls := make([]string, 0, 2)
	clearer := preferClearer(
		func() error {
			calls = append(calls, "primary")
			return errors.New("primary failed")
		},
		func() error {
			calls = append(calls, "fallback")
			return nil
		},
	)

	require.NoError(t, clearer())
	require.Equal(t, []string{"primary", "fallback"}, calls)
}

func TestPreferClearer_PrimarySuccessSkipsFallback(t *testing.T) {
	calls := make([]string, 0, 2)
	clearer := preferClearer(
		func() error {
			calls = append(calls, "primary")
			return nil
		},
		func() error {
			calls = append(calls, "fallback")
			return nil
		},
	)

	require.NoError(t, clearer())
	require.Equal(t, []string{"primary"}, calls)
}

func TestWlCopySetter_FallsBackWhenSensitiveFlagUnsupported(t *testing.T) {
	type call struct {
		text string
		args []string
	}

	calls := make([]call, 0, 2)
	errs := []error{
		&wlCopyCommandError{err: errors.New("exit status 1"), stderr: "wl-copy: unrecognized option '--sensitive'"},
		nil,
	}
	setter := wlCopySetterWithRunner(func(text string, args ...string) error {
		copied := append([]string(nil), args...)
		calls = append(calls, call{text: text, args: copied})
		err := errs[0]
		errs = errs[1:]
		return err
	})

	require.NoError(t, setter("secret"))
	require.Equal(t, []call{
		{text: "secret", args: []string{"--trim-newline", "--sensitive"}},
		{text: "secret", args: []string{"--trim-newline"}},
	}, calls)
}

func TestWlCopySetter_DoesNotRetryOnOtherErrors(t *testing.T) {
	calls := 0
	wantErr := &wlCopyCommandError{err: errors.New("exit status 1"), stderr: "permission denied"}
	setter := wlCopySetterWithRunner(func(text string, args ...string) error {
		calls++
		return wantErr
	})

	err := setter("secret")
	require.ErrorIs(t, err, wantErr)
	require.Equal(t, 1, calls)
}

func TestWlCopyClearer_PropagatesWrappedError(t *testing.T) {
	calls := make([][]string, 0, 1)
	wantErr := &wlCopyCommandError{err: errors.New("exit status 1"), stderr: "no clipboard"}
	clearer := wlCopyClearerWithRunner(func(args ...string) error {
		calls = append(calls, append([]string(nil), args...))
		return wantErr
	})

	err := clearer()
	require.ErrorIs(t, err, wantErr)
	require.Equal(t, [][]string{{"--clear"}}, calls)
}
