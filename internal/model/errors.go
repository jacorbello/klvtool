package model

import "fmt"

// Code identifies a shared extraction failure category.
type Code string

const (
	CodeInvalidUsage       Code = "invalid_usage"
	CodeMissingDependency  Code = "missing_dependency"
	CodeUnsupportedBackend Code = "unsupported_backend"
	CodeBackendExecution   Code = "backend_execution_failure"
	CodeBackendParse       Code = "backend_parse_failure"
	CodePacketParse        Code = "packet_parse_failure"
	CodeCheckpointRead     Code = "checkpoint_read_failure"
	CodeOutputWrite        Code = "output_write_failure"
	CodeTSSync             Code = "ts_sync_failure"
	CodeTSParse            Code = "ts_parse_failure"
	CodeTSRead             Code = "ts_read_failure"
	CodeKLVDecode          Code = "klv_decode_failure"
	CodeKLVSpec            Code = "klv_spec_unknown"
	CodeKLVChecksum        Code = "klv_checksum_mismatch"
	CodeKLVValidate        Code = "klv_validation_error"
)

// Error classifies a failure with a stable code and optional wrapped cause.
type Error struct {
	Code Code
	Err  error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return string(e.Code)
	}
	return fmt.Sprintf("%s: %v", e.Code, e.Err)
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *Error) Is(target error) bool {
	if e == nil {
		return target == nil
	}
	t, ok := target.(*Error)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

func NewError(code Code, err error) *Error {
	return &Error{Code: code, Err: err}
}

func InvalidUsage(err error) *Error {
	return NewError(CodeInvalidUsage, err)
}

func MissingDependency(err error) *Error {
	return NewError(CodeMissingDependency, err)
}

func UnsupportedBackend(err error) *Error {
	return NewError(CodeUnsupportedBackend, err)
}

func BackendExecution(err error) *Error {
	return NewError(CodeBackendExecution, err)
}

func BackendParse(err error) *Error {
	return NewError(CodeBackendParse, err)
}

func PacketParse(err error) *Error {
	return NewError(CodePacketParse, err)
}

func CheckpointRead(err error) *Error {
	return NewError(CodeCheckpointRead, err)
}

func OutputWrite(err error) *Error {
	return NewError(CodeOutputWrite, err)
}

func TSSync(err error) *Error {
	return NewError(CodeTSSync, err)
}

func TSParse(err error) *Error {
	return NewError(CodeTSParse, err)
}

func TSRead(err error) *Error {
	return NewError(CodeTSRead, err)
}

func KLVDecode(err error) *Error {
	return NewError(CodeKLVDecode, err)
}

func KLVSpec(err error) *Error {
	return NewError(CodeKLVSpec, err)
}

func KLVChecksum(err error) *Error {
	return NewError(CodeKLVChecksum, err)
}

func KLVValidate(err error) *Error {
	return NewError(CodeKLVValidate, err)
}
