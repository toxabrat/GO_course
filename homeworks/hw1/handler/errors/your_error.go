package errors

import "fmt"

type AdditionalMessageError struct {
	message string
	err     error
}

func NewAdditionalMessageError(err error, format string, args ...any) error {
	return &AdditionalMessageError{message: fmt.Sprintf(format, args...), err: err}
}

func (e *AdditionalMessageError) Error() string {
	if e.err == nil {
		return e.message
	}
	if e.message == "" {
		return e.err.Error()
	}
	return e.message + ": " + e.err.Error()
}

func (e *AdditionalMessageError) Unwrap() error {
	return e.err
}
