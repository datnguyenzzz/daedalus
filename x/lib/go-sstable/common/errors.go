package common

import "errors"

type CustomError struct {
	error
	code int
}

var ClientInvalidRequestError = CustomError{
	error: errors.New("client invalid request error"),
	code:  400,
}

var ClientContentTooLargeError = CustomError{
	error: errors.New("client is too large"),
	code:  413,
}

var InternalServerError = CustomError{
	error: errors.New("internal server error"),
	code:  500,
}

var MismatchedChecksumError = CustomError{
	error: errors.New("mismatched checksum"),
	code:  500,
}
