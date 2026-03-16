package websvc

import (
	"context"
	"fmt"
	"net/http"

	"github.com/acsl-go/logger"
	"github.com/acsl-go/service"
	"github.com/gin-gonic/gin"
)

func NewHandler(ctx context.Context, initializer func(context.Context, *gin.Engine)) http.Handler {
	if logger.Level >= logger.DEBUG {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	if logger.Level >= logger.DEBUG {
		router.Use(gin.Logger())
	}

	initializer(ctx, router)
	return router
}

// Deprecated: Use Server instead.
func Task(name string, config *Config, initializer func(context.Context, *gin.Engine)) service.ServiceTask {
	if config.SSLCert != "" && config.SSLKey != "" {
		if config.Port == 0 {
			config.Port = 443
		}
		return HttpsTask(name, fmt.Sprintf("%s:%d", config.Host, config.Port), config.SSLCert, config.SSLKey, initializer)
	} else {
		if config.Port == 0 {
			config.Port = 80
		}
		return HttpTask(name, fmt.Sprintf("%s:%d", config.Host, config.Port), initializer)
	}
}

// Deprecated: Use Server instead.
func HttpTask(name, addr string, initializer func(context.Context, *gin.Engine)) service.ServiceTask {
	return service.HttpServer(name, addr, func(ctx context.Context) http.Handler {
		return NewHandler(ctx, initializer)
	})
}

// Deprecated: Use Server instead.
func HttpsTask(name, addr, certFile, keyFile string, initializer func(context.Context, *gin.Engine)) service.ServiceTask {
	return service.HttpsServer(name, addr, certFile, keyFile, func(ctx context.Context) http.Handler {
		return NewHandler(ctx, initializer)
	})
}
