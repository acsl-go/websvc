package websvc

import "github.com/gin-gonic/gin"

func processResp(c *gin.Context, code int, rsp interface{}) {
	if rsp == nil {
		c.AbortWithStatus(code)
	} else if r, ok := rsp.(Response); ok {
		c.Data(code, r.ContentType, r.Body)
	} else if str, ok := rsp.(string); ok {
		c.String(code, str)
	} else {
		c.JSON(code, rsp)
	}
}
