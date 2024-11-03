package websvc

import (
	"fmt"
	"net/http"

	"github.com/acsl-go/logger"
	"github.com/acsl-go/service"
	"github.com/gin-gonic/gin"
)

func NewHandler(initializer func(*gin.Engine)) http.Handler {
	if logger.Level >= logger.DEBUG {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()

	if logger.Level >= logger.DEBUG {
		router.Use(gin.Logger())
	}

	initializer(router)
	return router
}

func Task(name string, config *Config, initializer func(*gin.Engine)) service.ServiceTask {
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

func HttpTask(name, addr string, initializer func(*gin.Engine)) service.ServiceTask {
	return service.HttpServer(name, addr, NewHandler(initializer))
}

func HttpsTask(name, addr, certFile, keyFile string, initializer func(*gin.Engine)) service.ServiceTask {
	return service.HttpsServer(name, addr, certFile, keyFile, NewHandler(initializer))
}
