package websvc

import "errors"

var (
	ErrInvalidTlsKeyPair  = errors.New("invalid TLS key pair")
	ErrPortBindingFailed  = errors.New("failed to bind to the specified port")
	ErrInvalidPrivateKey  = errors.New("invalid private key")
	ErrInvalidCertificate = errors.New("invalid certificate")
	ErrIOFailure          = errors.New("I/O failure")
)
