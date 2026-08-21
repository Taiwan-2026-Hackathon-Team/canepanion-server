package errors

import "fmt"

type AppError struct {
	Code    int
	Message string
	Err     error
}

func (e *AppError) Error() string {
	return fmt.Sprintf("%s %v", e.Message, e.Err)
}

func NewBadRequest(msg string, err error) *AppError {
	return &AppError{Code: 400, Message: msg, Err: err}
}

func NewNotFound(msg string, err error) *AppError {
	return &AppError{Code: 404, Message: msg, Err: err}
}

func NewUnauthorized(msg string, err error) *AppError {
	return &AppError{Code: 401, Message: msg, Err: err}
}

func NewForbidden(msg string, err error) *AppError {
	return &AppError{Code: 403, Message: msg, Err: err}
}

func NewInternal(msg string, err error) *AppError {
	return &AppError{Code: 500, Message: msg, Err: err}
}

func NewMethodNotAllowed(msg string, err error) *AppError {
	return &AppError{Code: 405, Message: msg, Err: err}
}

func NewNotAcceptable(msg string, err error) *AppError {
	return &AppError{Code: 406, Message: msg, Err: err}
}

func NewContentTooLarge(msg string, err error) *AppError {
	return &AppError{Code: 413, Message: msg, Err: err}
}

func NewUnsupportedMediaType(msg string, err error) *AppError {
	return &AppError{Code: 415, Message: msg, Err: err}
}

func NewUnprocessableEntity(msg string, err error) *AppError {
	return &AppError{Code: 422, Message: msg, Err: err}
}

func NewTooManyRequests(msg string, err error) *AppError {
	return &AppError{Code: 429, Message: msg, Err: err}
}

func NewGatewayTimeout(msg string, err error) *AppError {
	return &AppError{Code: 504, Message: msg, Err: err}
}
