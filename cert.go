package websvc

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"
)

func loadData(fileOrData string) ([]byte, error) {
	// Attempt to load from file first
	if _, err := os.Stat(fileOrData); err == nil {
		return os.ReadFile(fileOrData)
	}
	// If file does not exist, treat it as data
	return []byte(fileOrData), nil
}

func LoadX509KeyPair(certFile, keyFile string) (*tls.Certificate, []*x509.Certificate, error) {
	certs, raw, err := LoadCertsFromPEM(certFile)
	if err != nil {
		return nil, nil, err
	}

	key, err := LoadPrivateKeyFromPEM(keyFile)
	if err != nil {
		return nil, nil, err
	}

	return &tls.Certificate{
		Certificate: raw,
		PrivateKey:  key,
		Leaf:        certs[0],
	}, certs, nil
}

func LoadCertsFromPEM(fileOrData string) ([]*x509.Certificate, [][]byte, error) {
	// Attempt to load from file first
	if _, err := os.Stat(fileOrData); err == nil {
		return LoadCertsFromPEMFile(fileOrData)
	}
	// If file does not exist, treat it as PEM data
	return LoadCertsFromPEMData([]byte(fileOrData))
}

func LoadCertsFromPEMData(certData []byte) ([]*x509.Certificate, [][]byte, error) {
	certs := []*x509.Certificate{}
	raw := [][]byte{}
	for {
		certBlock, certData := pem.Decode(certData)
		if certBlock == nil || certBlock.Type != "CERTIFICATE" {
			return nil, nil, ErrInvalidCertificate
		}
		cert, err := x509.ParseCertificate(certBlock.Bytes)
		if err != nil {
			return nil, nil, ErrInvalidCertificate
		}
		certs = append(certs, cert)
		raw = append(raw, certBlock.Bytes)
		if len(certData) == 0 {
			break
		}
	}
	return certs, raw, nil
}

func LoadCertsFromPEMFile(certFile string) ([]*x509.Certificate, [][]byte, error) {
	certData, err := os.ReadFile(certFile)
	if err != nil {
		return nil, nil, ErrIOFailure
	}
	return LoadCertsFromPEMData(certData)
}

func LoadPrivateKeyFromPEM(fileOrData string) (interface{}, error) {
	// Attempt to load from file first
	if _, err := os.Stat(fileOrData); err == nil {
		return LoadPrivateKeyFromPEMFile(fileOrData)
	}
	// If file does not exist, treat it as PEM data
	return LoadPrivateKeyFromPEMData([]byte(fileOrData))
}

func LoadPrivateKeyFromPEMData(keyData []byte) (interface{}, error) {
	keyBlock, _ := pem.Decode(keyData)
	if keyBlock == nil || (keyBlock.Type != "RSA PRIVATE KEY" && keyBlock.Type != "EC PRIVATE KEY" && keyBlock.Type != "PRIVATE KEY") {
		return nil, ErrInvalidPrivateKey
	}
	keyData = keyBlock.Bytes

	// RSA PKCS1 Private Key
	if key, err := x509.ParsePKCS1PrivateKey(keyData); err == nil {
		return key, nil
	}
	// RSA PKCS8 Private Key
	if key, err := x509.ParsePKCS8PrivateKey(keyData); err == nil {
		return key, nil
	}
	// ECDSA Private Key
	if key, err := x509.ParseECPrivateKey(keyData); err == nil {
		return key, nil
	}
	return nil, ErrInvalidPrivateKey
}

func LoadPrivateKeyFromPEMFile(keyFile string) (interface{}, error) {
	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, ErrIOFailure
	}
	return LoadPrivateKeyFromPEMData(keyData)
}
