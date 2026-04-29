package errors

import "errors"

type Kind string

const (
	KindUnauthenticated Kind = "unauthenticated"
	KindLocked          Kind = "locked"
	KindNotFound        Kind = "not_found"
	KindConflict        Kind = "conflict"
	KindValidation      Kind = "validation"
	KindCrypto          Kind = "crypto"
	KindNetwork         Kind = "network"
	KindTemporary       Kind = "temporary"
	KindUnsupported     Kind = "unsupported"
)

type Error struct {
	Kind    Kind
	Op      string
	Code    string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Op == "" {
		return e.Message
	}
	return e.Op + ": " + e.Message
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func (e *Error) Is(target error) bool {
	var other *Error
	if !errors.As(target, &other) || e == nil || other == nil {
		return false
	}
	if e.Kind == "" || other.Kind == "" {
		return false
	}
	return e.Kind == other.Kind
}

var (
	ErrLocked          = &Error{Kind: KindLocked}
	ErrConflict        = &Error{Kind: KindConflict}
	ErrNotFound        = &Error{Kind: KindNotFound}
	ErrUnsupported     = &Error{Kind: KindUnsupported}
	ErrUnauthenticated = &Error{Kind: KindUnauthenticated}
)
