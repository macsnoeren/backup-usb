package app

import (
	"errors"
	"fmt"
)

// Kind classifies an error. internal/cli maps kinds to exit codes; nothing
// else knows the numbers.
type Kind int

// Error kinds, in the order of the exit code table in the design.
const (
	KindNone        Kind = iota
	KindWarning          // succeeded with warnings
	KindNoDisk           // no backup disk connected
	KindWrongDisk        // wrong disk for this period
	KindSkipped          // recently done already
	KindBusy             // another instance is running
	KindUnknownDisk      // ID file, signature or serials do not match
	KindTampered         // settings or registrations without valid signature
	KindDiskCheck        // space, file system, writable, missing key file
	KindIO               // read or write error
	KindVerify           // backup verification failed
	KindDecrypt          // decryption or rewrap failed
	KindUsage            // wrong usage
)

// Error is an error with a kind and a user-facing message key from the
// translation table (internal/msg). Err is the technical cause, in English,
// and never contains secrets.
type Error struct {
	Kind Kind
	Key  string
	Args []any
	Err  error
}

// Error returns a technical description for logs.
func (e *Error) Error() string {
	s := e.Key
	if len(e.Args) > 0 {
		s += fmt.Sprintf(" %v", e.Args)
	}
	if e.Err != nil {
		s += ": " + e.Err.Error()
	}
	return s
}

// Unwrap returns the cause.
func (e *Error) Unwrap() error { return e.Err }

// Errorf returns an *Error of the given kind with message key and args.
func Errorf(kind Kind, cause error, key string, args ...any) *Error {
	return &Error{Kind: kind, Key: key, Args: args, Err: cause}
}

// KindOf returns the kind of err: the kind of the first *Error in the chain,
// KindIO for any other non-nil error and KindNone for nil.
func KindOf(err error) Kind {
	if err == nil {
		return KindNone
	}
	var e *Error
	if errors.As(err, &e) {
		return e.Kind
	}
	return KindIO
}
