package agent

import "fmt"

type (
	InvalidTokenErr struct {
		msg string
		err error
	}
)

func NewInvalidTokenErr(msg string) *InvalidTokenErr {
	return &InvalidTokenErr{
		msg: msg,
	}
}

func NewInvalidTokenErrf(msg string, a ...any) *InvalidTokenErr {
	err := fmt.Errorf(msg, a...)
	return &InvalidTokenErr{
		msg: err.Error(),
		err: err,
	}
}

func (i *InvalidTokenErr) Error() string {
	return i.msg
}

func (i *InvalidTokenErr) Unwrap() error {
	return i.err
}
