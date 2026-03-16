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

func loadX509KeyPair(certFile, keyFile string) (*tls.Certificate, []*x509.Certificate, error) {
	certs, err := loadCertsFromPEM(certFile)
	if err != nil {
		return nil, nil, err
	}

	key, err := loadPrivateKeyFromPEM(keyFile)
	if err != nil {
		return nil, nil, err
	}

	return &tls.Certificate{
		Certificate: [][]byte{},
		PrivateKey:  key,
		Leaf:        certs[0],
	}, certs, nil
}

func loadCertsFromPEM(fileOrData string) ([]*x509.Certificate, error) {
	// Attempt to load from file first
	if _, err := os.Stat(fileOrData); err == nil {
		return loadCertsFromPEMFile(fileOrData)
	}
	// If file does not exist, treat it as PEM data
	return loadCertsFromPEMData([]byte(fileOrData))
}

func loadCertsFromPEMData(certData []byte) ([]*x509.Certificate, error) {
	certs := []*x509.Certificate{}
	for {
		certBlock, certData := pem.Decode(certData)
		if certBlock == nil || certBlock.Type != "CERTIFICATE" {
			return nil, ErrInvalidCertificate
		}
		cert, err := x509.ParseCertificate(certBlock.Bytes)
		if err != nil {
			return nil, ErrInvalidCertificate
		}
		certs = append(certs, cert)
		if len(certData) == 0 {
			break
		}
	}
	return certs, nil
}

func loadCertsFromPEMFile(certFile string) ([]*x509.Certificate, error) {
	certData, err := os.ReadFile(certFile)
	if err != nil {
		return nil, ErrIOFailure
	}
	return loadCertsFromPEMData(certData)
}

func loadPrivateKeyFromPEM(fileOrData string) (interface{}, error) {
	// Attempt to load from file first
	if _, err := os.Stat(fileOrData); err == nil {
		return loadPrivateKeyFromPEMFile(fileOrData)
	}
	// If file does not exist, treat it as PEM data
	return loadPrivateKeyFromPEMData([]byte(fileOrData))
}

func loadPrivateKeyFromPEMData(keyData []byte) (interface{}, error) {
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

func loadPrivateKeyFromPEMFile(keyFile string) (interface{}, error) {
	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, ErrIOFailure
	}
	return loadPrivateKeyFromPEMData(keyData)
}
