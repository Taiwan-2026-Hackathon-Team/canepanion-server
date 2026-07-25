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

func NewInternal(msg string, err error) *AppError {
	return &AppError{Code: 500, Message: msg, Err: err}
}
