package websvc

import "errors"

var (
	ErrInvalidTlsKeyPair = errors.New("invalid TLS key pair")
	ErrPortBindingFailed = errors.New("failed to bind to the specified port")
)
