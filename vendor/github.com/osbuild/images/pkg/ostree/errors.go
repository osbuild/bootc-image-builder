package ostree

import "fmt"

// ResolveRefError is returned when there is a failure to resolve the
// reference.
type ResolveRefError struct {
	msg string
}

func (e ResolveRefError) Error() string {
	return e.msg
}

// NewResolveRefError creates and returns a new ResolveRefError with a given
// formatted message.
func NewResolveRefError(msg string, args ...interface{}) ResolveRefError {
	return ResolveRefError{msg: fmt.Sprintf(msg, args...)}
}

// RefError is returned when a parameter is invalid (e.g., malformed
// or contains illegal characters).
type RefError struct {
	msg string
}

func (e RefError) Error() string {
	return e.msg
}

// NewRefError creates and returns a new InvalidParameterError
// with a given formatted message.
func NewRefError(msg string, args ...interface{}) RefError {
	return RefError{msg: fmt.Sprintf(msg, args...)}
}

type ParameterComboError struct {
	msg string
}

func (e ParameterComboError) Error() string {
	return e.msg
}

func NewParameterComboError(msg string, args ...interface{}) ParameterComboError {
	return ParameterComboError{msg: fmt.Sprintf(msg, args...)}
}
