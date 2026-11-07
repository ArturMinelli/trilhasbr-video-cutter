package apperrors

import (
	"errors"
	"fmt"
)

type Code string

const (
	CodeValidation     Code = "VALIDATION_ERROR"
	CodeDB             Code = "DB_ERROR"
	CodeFFmpegNotFound Code = "FFMPEG_NOT_FOUND"
	CodeProbe          Code = "PROBE_ERROR"
	CodeCut            Code = "CUT_ERROR"
	CodeMerge          Code = "MERGE_ERROR"
	CodeIO             Code = "IO_ERROR"
	CodeInternal       Code = "INTERNAL_ERROR"
	CodeModelNotFound  Code = "MODEL_NOT_FOUND"
	CodeInference      Code = "INFERENCE_ERROR"
	CodeDetection      Code = "DETECTION_ERROR"
)

type AppError struct {
	Code    Code           `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
	Err     error          `json:"-"`
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	if e.Err == nil {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func New(code Code, message string, details map[string]any) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Details: details,
	}
}

func Wrap(code Code, message string, err error, details map[string]any) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
		Details: details,
		Err:     err,
	}
}

func As(err error) *AppError {
	if err == nil {
		return nil
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return &AppError{
		Code:    CodeInternal,
		Message: "unexpected internal error",
		Err:     err,
	}
}

type PublicError struct {
	Code    Code           `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

func ToPublic(err error) *PublicError {
	if err == nil {
		return nil
	}
	appErr := As(err)
	return &PublicError{
		Code:    appErr.Code,
		Message: appErr.Message,
		Details: appErr.Details,
	}
}
