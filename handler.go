package websvc

import (
	"github.com/acsl-go/logger"
	"github.com/gin-gonic/gin"
)

func Handler[TSES interface{}](handler func(*gin.Context) (int, interface{}, error), privilege string) gin.HandlerFunc {
	return func(c *gin.Context) {
		code, rsp, e := handler(c)
		if e != nil {
			logger.Error("Error: %+v", e)
			c.AbortWithStatus(500)
			return
		}

		processResp(c, code, rsp)
	}
}

func HandlerD[TSES interface{}, TDATA interface{}](handler func(*gin.Context, TDATA) (int, interface{}, error), privilege string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var data TDATA
		if e := c.ShouldBindJSON(&data); e != nil {
			logger.Error("Error: %+v", e)
			c.AbortWithStatus(400)
			return
		}

		code, rsp, e := handler(c, data)
		if e != nil {
			logger.Error("Error: %+v", e)
			c.AbortWithStatus(500)
			return
		}

		processResp(c, code, rsp)
	}
}

func HandlerQ[TSES interface{}](handler func(*gin.Context, map[string]string) (int, interface{}, error), privilege string) gin.HandlerFunc {
	return func(c *gin.Context) {

		queries, _, e := parseQuery(c)
		if e != nil {
			c.AbortWithStatus(400)
			return
		}

		code, rsp, e := handler(c, queries)
		if e != nil {
			logger.Error("Error: %+v", e)
			c.AbortWithStatus(500)
			return
		}

		processResp(c, code, rsp)
	}
}

func HandlerQD[TSES interface{}, TDATA interface{}](handler func(*gin.Context, map[string]string, TDATA) (int, interface{}, error), privilege string) gin.HandlerFunc {
	return func(c *gin.Context) {
		queries, _, e := parseQuery(c)
		if e != nil {
			c.AbortWithStatus(400)
			return
		}

		var data TDATA
		if e := c.ShouldBindJSON(&data); e != nil {
			logger.Error("Error: %+v", e)
			c.AbortWithStatus(400)
			return
		}

		code, rsp, e := handler(c, queries, data)
		if e != nil {
			logger.Error("Error: %+v", e)
			c.AbortWithStatus(500)
			return
		}

		processResp(c, code, rsp)
	}
}
