package errors

import (
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrorRedactsCause(t *testing.T) {
	secretCause := stderrors.New("password=secret")
	err := &Error{Kind: KindValidation, Op: "config.Validate", Code: "invalid", Message: "invalid config", Cause: secretCause}
	require.Equal(t, "config.Validate: invalid config", err.Error())
	require.NotContains(t, err.Error(), "secret")
	require.ErrorIs(t, err, secretCause)
}

func TestErrorIsKind(t *testing.T) {
	err := &Error{Kind: KindLocked, Op: "app.Search", Message: "locked"}
	require.True(t, stderrors.Is(err, ErrLocked))
	require.False(t, stderrors.Is(err, ErrConflict))
}
