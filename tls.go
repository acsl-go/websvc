package websvc

import (
	"crypto/x509"

	"github.com/gin-gonic/gin"
)

type TLSCertVerifier struct {
	opts x509.VerifyOptions
}

func NewTLSCertVerifier(trustedCAs []string) *TLSCertVerifier {
	cas := x509.NewCertPool()
	for _, ca := range trustedCAs {
		cacert, e := loadData(ca)
		if e == nil && cacert != nil {
			cas.AppendCertsFromPEM(cacert)
		}
	}
	return &TLSCertVerifier{
		opts: x509.VerifyOptions{
			Roots:     cas,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		},
	}
}

func (v *TLSCertVerifier) Verify(cert *x509.Certificate) error {
	_, e := cert.Verify(v.opts)
	return e
}

func NewTLSCertChecker(checker func(*gin.Context, *x509.Certificate) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.TLS == nil || len(c.Request.TLS.PeerCertificates) == 0 {
			c.String(400, "TLS client certificate required")
			c.Abort()
			return
		}

		clientCert := c.Request.TLS.PeerCertificates[0]
		err := checker(c, clientCert)
		if err != nil {
			c.String(403, err.Error())
			c.Abort()
			return
		}

		c.Next()
	}
}
