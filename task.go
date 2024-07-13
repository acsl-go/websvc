package websvc

import (
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

func Task(name, addr string, initializer func(*gin.Engine)) service.ServiceTask {
	return service.HttpServer(name, addr, NewHandler(initializer))
}

func HttpTask(name, addr string, initializer func(*gin.Engine)) service.ServiceTask {
	return service.HttpServer(name, addr, NewHandler(initializer))
}

func HttpsTask(name, addr, certFile, keyFile string, initializer func(*gin.Engine)) service.ServiceTask {
	return service.HttpsServer(name, addr, certFile, keyFile, NewHandler(initializer))
}
