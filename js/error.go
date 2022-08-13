package js

/*
import "rogchap.com/v8go"

const (
	v8Err = iota
	unkErr
)

//JSError provides a JS runtime agnostic error
type JSError struct {
	errType int
	err     error
}

func (j JSError) Error() string {
	return j.err.Error()
}

func (j JSError) StackTrace() string {
	if j.errType == v8Err {
		return j.err.(*v8go.JSError).StackTrace
	}
	return ""
}

func newV8JSError(err error) JSError {
	return JSError{
		errType: v8Err,
		err:     err,
	}
}
*/
