package websvc

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/acsl-go/logger"
	"github.com/acsl-go/service"
	"github.com/gin-gonic/gin"
)

type Server struct {
	name        string
	config      *Config
	initializer func(context.Context, *gin.Engine)
	tlsConfig   *tls.Config
	listener    net.Listener
	router      http.Handler
	server      *http.Server

	Host string // Actual listen host
	Port int    // Actual listen port, may be different from config if config.Port is 0
	TLS  bool   // Whether TLS is enabled
}

func NewServer(name string, config *Config, initializer func(context.Context, *gin.Engine)) *Server {
	return &Server{
		name:        name,
		config:      config,
		initializer: initializer,
	}
}

// Listen starts the server and binds to the specified port. It should be called before starting the service.
func (s *Server) Listen() error {
	// Do listen first to catch errors before starting the service
	if s.config.IsSSL() {
		s.tlsConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		cert, err := tls.LoadX509KeyPair(s.config.SSLCert, s.config.SSLKey)
		if err != nil {
			return ErrInvalidTlsKeyPair
		}
		s.tlsConfig.Certificates = []tls.Certificate{cert}

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
	s.server.Handler = NewHandler(ctx, s.initializer)
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
func NewServerTask(name string, config *Config, initializer func(context.Context, *gin.Engine), retryDuration time.Duration) service.ServiceTask {
	server := NewServer(name, config, initializer)
	return server.Task(retryDuration)
}
