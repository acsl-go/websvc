package websvc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/acsl-go/logger"
	"github.com/acsl-go/service"
	"github.com/gin-gonic/gin"
)

type ServerInitializer func(context.Context, *gin.Engine, *Server)

type Server struct {
	name        string
	config      *Config
	initializer ServerInitializer
	tlsConfig   *tls.Config
	listener    net.Listener
	router      http.Handler
	server      *http.Server

	Host string // Actual listen host
	Port int    // Actual listen port, may be different from config if config.Port is 0
	TLS  bool   // Whether TLS is enabled

	Attachment interface{}
}

func NewServer(name string, config *Config, initializer ServerInitializer, attachment interface{}) *Server {
	return &Server{
		name:        name,
		config:      config,
		initializer: initializer,
		Attachment:  attachment,
	}
}

// Listen starts the server and binds to the specified port. It should be called before starting the service.
func (s *Server) Listen() error {
	// Do listen first to catch errors before starting the service
	if s.config.IsSSL() {
		s.tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		cert, _, err := loadX509KeyPair(s.config.SSLCert, s.config.SSLKey)
		if err != nil {
			return ErrInvalidTlsKeyPair
		}
		s.tlsConfig.Certificates = []tls.Certificate{*cert}

		switch s.config.ClientAuthType {
		case "none":
			s.tlsConfig.ClientAuth = tls.NoClientCert // No client certificate required
		case "optional":
			s.tlsConfig.ClientAuth = tls.RequestClientCert // Client certificate is requested but not required, will be verified if provided
		case "required":
			s.tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert // Client certificate is required but will not be verified automatically, verification should be done in the application code using the VerifyPeerCertificate callback to allow for custom verification logic (e.g. support for self-signed certificates)
		case "must":
			s.tlsConfig.ClientAuth = tls.RequireAnyClientCert // Client certificate is required and must be verified by the RootCAs pool, will be verified automatically by the TLS stack using the RootCAs field of the TLS config (which can be set to a custom CA pool if needed)
		default:
			s.tlsConfig.ClientAuth = tls.NoClientCert
			logger.Warn("Invalid client_auth_type: %s, defaulting to 'none'", s.config.ClientAuthType)
		}

		if s.tlsConfig.ClientAuth != tls.NoClientCert {
			s.tlsConfig.ClientCAs = x509.NewCertPool()
			for _, ca := range s.config.CACerts {
				cacert, err := loadData(ca)
				if err != nil {
					return fmt.Errorf("failed to load CA certificate: %w", err)
				}
				s.tlsConfig.ClientCAs.AppendCertsFromPEM(cacert)
			}
		}

		listener, err := tls.Listen("tcp", fmt.Sprintf("%s:%d", s.config.Host, s.config.Port), s.tlsConfig)
		if err != nil {
			return ErrPortBindingFailed
		}

		s.listener = listener
		s.TLS = true
	} else {
		listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", s.config.Host, s.config.Port))
		if err != nil {
			return ErrPortBindingFailed
		}
		s.listener = listener
		s.TLS = false
	}

	// Retrieve the actual port in case it was set to 0 (random)
	addr := s.listener.Addr().(*net.TCPAddr)
	s.Host = addr.IP.String()
	s.Port = addr.Port

	// Create the HTTP server with the router as the handler
	s.server = &http.Server{
		Addr:      fmt.Sprintf("%s:%d", s.Host, s.Port),
		TLSConfig: s.tlsConfig,
		Handler:   s.router,
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// Wrap the server's Listen and Serve in one call
func (s *Server) Start(ctx context.Context) error {
	if s.server == nil {
		err := s.Listen()
		if err != nil {
			return err
		}
	}

	if logger.Level >= logger.DEBUG {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	if logger.Level >= logger.DEBUG {
		router.Use(gin.Logger())
	}
	s.initializer(ctx, router, s)

	s.server.Handler = router
	go s.server.Serve(s.listener)
	return nil
}

// Run the server as a service task
// The task will attempt to start the server, and if it fails (e.g. due to port binding issues), it will log a warning and retry until it succeeds or the context is canceled.
func (s *Server) Task(retryDuration time.Duration) service.ServiceTask {
	return func(ctx context.Context) {
		for {
			err := s.Start(ctx)
			if err == nil {
				break // Started successfully
			}
			logger.Warn("Failed to start server: %v, will retry in %v", err, retryDuration)
			select {
			case <-ctx.Done():
				return
			case <-time.After(retryDuration):
				// Retry after delay
			}
		}
		if s.TLS {
			logger.Info("HTTPS server %s started on %s:%d\n", s.name, s.Host, s.Port)
		} else {
			logger.Info("HTTP server %s started on %s:%d\n", s.name, s.Host, s.Port)
		}
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.server.Shutdown(c); err != nil {
			logger.Error("Shutdown error:  %+v\n", err)
		} else {
			logger.Info("Server %s on %s:%d stopped gracefully\n", s.name, s.Host, s.Port)
		}
	}
}

// Create a new server task with the given configuration and initializer
func NewServerTask(name string, config *Config, initializer ServerInitializer, retryDuration time.Duration, attachment interface{}) service.ServiceTask {
	server := NewServer(name, config, initializer, attachment)
	return server.Task(retryDuration)
}
