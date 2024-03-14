package websvc

import (
	"github.com/acsl-go/logger"
	"github.com/acsl-go/service"
	"github.com/gin-gonic/gin"
)

func Task(name, addr string, initializer func(*gin.Engine)) service.ServiceTask {
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
	return service.HttpServer(name, addr, router)
}
