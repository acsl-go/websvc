package websvc

import (
	"crypto/x509"

	"github.com/gin-gonic/gin"
)

type TLSCertVerificationConfig struct {
	TrustedCAs []string // The trusted CA certificates for TLS connections

	// [Optional] Custom certificate checker
	// if specified, will be used to verify the remote TLS certificate, return nil if the certificate is valid, otherwise return an error
	// The error message will be pass to the client with HTTP status code 403
	// if not specified, the checker will only check if the certificate is signed by one of the trusted CAs, and has the correct key usage for client authentication
	CertChecker func(*gin.Context, *x509.Certificate) error
}

func NewTLSCertVerifier(cfg *TLSCertVerificationConfig) gin.HandlerFunc {
	cas := x509.NewCertPool()
	for _, ca := range cfg.TrustedCAs {
		cas.AppendCertsFromPEM([]byte(ca))
	}

	opts := x509.VerifyOptions{
		Roots:     cas,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	certChecker := cfg.CertChecker
	if certChecker == nil {
		certChecker = func(c *gin.Context, cert *x509.Certificate) error {
			return nil
		}
	}

	return func(c *gin.Context) {
		if c.Request.TLS == nil || len(c.Request.TLS.PeerCertificates) == 0 {
			c.String(400, "TLS client certificate required")
			c.Abort()
			return
		}

		clientCert := c.Request.TLS.PeerCertificates[0]
		_, err := clientCert.Verify(opts)
		if err != nil {
			c.String(403, "Invalid TLS client certificate")
			c.Abort()
			return
		}

		err = certChecker(c, clientCert)
		if err != nil {
			c.String(403, err.Error())
			c.Abort()
			return
		}

		c.Next()
	}
}
