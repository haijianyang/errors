package errors

import (
	"fmt"
	"io"
	"sync"
)

// Coder defines an interface for an error code detail information.
type Coder interface {
	// Code returns the code of the coder
	Code() string

	// HTTP status that should be used for the associated error code.
	StatusCode() int

	// External (user) facing error message.
	Message() string

	// Params returns the params of message.
	Params() map[string]interface{}

	// FullMessage returns the message with params.
	FullMessage() string

	// Reference returns the detail documents for user.
	Reference() string
}

// codes contains a map of error codes to metadata.
var codes = map[string]Coder{}
var codeMux = &sync.Mutex{}

// Register register a user define error code.
// It will overrid the exist code.
func Register(coder Coder) {
	codeMux.Lock()
	defer codeMux.Unlock()

	codes[coder.Code()] = coder
}

// MustRegister register a user define error code.
// It will panic when the same Code already exist.
func MustRegister(coder Coder) {
	codeMux.Lock()
	defer codeMux.Unlock()

	if _, ok := codes[coder.Code()]; ok {
		panic(fmt.Sprintf("code: %s already exist", coder.Code()))
	}

	codes[coder.Code()] = coder
}

// GetCoder return the coder by code.
func GetCoder(code string) Coder {
	if coder, ok := codes[code]; ok {
		return coder
	}

	return nil
}

// ParseCoder parse any error into *withCode.
// nil error will return nil direct.
// None withCode error will be parsed as nil.
func ParseCoder(err error) Coder {
	if err == nil {
		return nil
	}

	if wc, ok := err.(*withCode); ok {
		if coder, ok := codes[wc.code]; ok {
			return coder
		}
	}

	return nil
}

// IsCode reports whether the error's code is the given code.
func IsCode(err error, code string) bool {
	if coder, ok := err.(*withCode); ok {
		if coder.code == code {
			return true
		}
	}

	return false
}

// HasCode reports whether any error in err's chain contains the given error code.
func HasCode(err error, code string) bool {
	if coder, ok := err.(*withCode); ok {
		if coder.code == code {
			return true
		}

		if coder.cause != nil {
			return HasCode(coder.cause, code)
		}

		return false
	}

	return false
}

type withCode struct {
	code    string
	message string
	params  map[string]interface{}
	cause   error
	*stack
}

func (w *withCode) Code() string { return w.code }

func (w *withCode) Message() string { return w.message }

func (w *withCode) Params() map[string]interface{} { return w.params }

func (w *withCode) Cause() error { return w.cause }

func (w *withCode) Error() string {
	errString := w.code + " - " + w.message
	if w.cause != nil {
		errString += ": " + w.cause.Error()
	}

	return errString
}

// Unwrap provides compatibility for Go 1.13 error chains.
func (w *withCode) Unwrap() error { return w.cause }

func (w *withCode) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') {
			if w.Cause() != nil {
				fmt.Fprintf(s, "%+v\n", w.Cause())
			}

			io.WriteString(s, w.code+" - "+w.message)
			w.stack.Format(s, verb)
			return
		}
		fallthrough
	case 's', 'q':
		io.WriteString(s, w.Error())
	}
}

// Code returns the underlying code of the error, if possible.
// An error value has a cause if it implements the following
// interface:
//
//     type coder interface {
//            Code() error
//     }
//
// If the error does not implement Code or the error is nil,
// the empty string will be returned.
func Code(err error) string {
	code := ""
	if err == nil {
		return code
	}

	type coder interface {
		Code() string
	}

	cd, ok := err.(coder)
	if ok {
		code = cd.Code()
	}

	return code
}

// Message returns the underlying message of the error, if possible.
// An error value has a cause if it implements the following
// interface:
//
//     type messager interface {
//            Message() error
//     }
//
// If the error does not implement Message or the error is nil,
// the empty string will be returned.
func Message(err error) string {
	msg := ""
	if err == nil {
		return msg
	}

	type messager interface {
		Message() string
	}

	msger, ok := err.(messager)
	if ok {
		msg = msger.Message()
	}

	return msg
}

// FullMessage returns the underlying full message of the error, if possible.
// An error value has a cause if it implements the following
// interface:
//
//     type fullmessager interface {
//            FullMessage() error
//     }
//
// If the error does not implement FullMessage or the error is nil,
// the empty string will be returned.
func FullMessage(err error) string {
	fullMsg := ""
	if err == nil {
		return fullMsg
	}

	type fullmessager interface {
		FullMessage() string
	}

	fullMsger, ok := err.(fullmessager)
	if ok {
		fullMsg = fullMsger.FullMessage()
	}

	return fullMsg
}

// Params returns the underlying params of the error, if possible.
// An error value has a cause if it implements the following
// interface:
//
//     type parameter interface {
//            Params() error
//     }
//
// If the error does not implement parameter or the error is nil,
// the nil will be returned.
func Params(err error) map[string]interface{} {
	if err == nil {
		return nil
	}

	type parameter interface {
		Params() map[string]interface{}
	}

	paramer, ok := err.(parameter)
	if ok {
		return paramer.Params()
	}

	return nil
}

// NewCode returns an error with the supplied code and message.
// NewCode also records the stack trace at the point it was called.
func NewCode(code string, msgs ...string) error {
	return &withCode{
		code:    code,
		message: message(code, msgs),
		stack:   callers(),
	}
}

func NewCodeWithParams(code string, params map[string]interface{}, msgs ...string) error {
	return &withCode{
		code:    code,
		message: message(code, msgs),
		params:  params,
		stack:   callers(),
	}
}

// WrapCode returns an error annotating err with a stack trace
// at the point WrapCode is called, and the supplied code and message.
// If err is nil, WrapCode returns nil.
func WrapCode(err error, code string, msgs ...string) error {
	if err == nil {
		return nil
	}

	return &withCode{
		code:    code,
		message: message(code, msgs),
		cause:   err,
		stack:   callers(),
	}
}

func WrapCodeWithParams(err error, code string, params map[string]interface{}, msgs ...string) error {
	if err == nil {
		return nil
	}

	return &withCode{
		code:    code,
		message: message(code, msgs),
		params:  params,
		cause:   err,
		stack:   callers(),
	}
}

func message(code string, msgs []string) string {
	message := ""
	if len(msgs) == 0 {
		if coder, ok := codes[code]; ok {
			message = coder.Message()
		}
	} else {
		message = msgs[0]
	}

	return message
}
