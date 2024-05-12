package websvc

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/acsl-go/logger"
	"github.com/gin-gonic/gin"
)

func doAuthN(c *gin.Context, cfg *AuthenticatorConfigure, privilege string) (interface{}, map[string]string, int) {
	auth_str := c.GetHeader("Authorization")
	if auth_str == "" {
		return nil, nil, 401
	}

	auth_parts := strings.Split(strings.ReplaceAll(auth_str, "Bearer ", ""), ":")
	if len(auth_parts) != 3 {
		return nil, nil, 401
	}

	ses_id := auth_parts[0]
	signature := auth_parts[1]
	timestamp, e := strconv.ParseInt(auth_parts[2], 10, 64)
	if e != nil {
		return nil, nil, 401
	}

	ts_error := time.Now().UnixMilli() - timestamp
	if ts_error > cfg.TsTolerance || ts_error < -cfg.TsTolerance {
		return nil, nil, 400
	}

	ses, token, e := cfg.QueryToken(c, ses_id)
	if e != nil {
		logger.Error("QueryToken Error: %+v", e)
		return nil, nil, 500
	}

	if ses == nil || token == "" {
		return nil, nil, 401
	}

	queries, query, e := parseQuery(c)
	if e != nil {
		return nil, nil, 400
	}

	sig_str := auth_parts[0] + ":" + token + ":" + auth_parts[2] + ":" + query
	sig_hash := sha256.Sum256([]byte(sig_str))
	if signature != hex.EncodeToString(sig_hash[:]) {
		return nil, nil, 401
	}

	if cfg.CheckPrivilege != nil && privilege != "" {
		if e := cfg.CheckPrivilege(c, ses_id, ses, privilege); e != nil {
			if errors.Is(e, ErrNoPrivilege) {
				return nil, nil, 403
			} else {
				logger.Fatal("privilege checking failed: %+v", e)
				return nil, nil, 500
			}
		}
	}

	if cfg.RefreshToken != nil {
		if e := cfg.RefreshToken(c, ses_id, ses); e != nil {
			logger.Fatal("token refresh failed: %+v", e)
			return nil, nil, 500
		}
	}

	return ses, queries, 0
}

func AuthN[TSES interface{}](cfg *AuthenticatorConfigure, handler func(*gin.Context, TSES) (int, interface{}, error), privilege string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ses_obj, _, code := doAuthN(c, cfg, privilege)
		if code != 0 {
			c.AbortWithStatus(code)
			return
		}

		ses, ok := ses_obj.(TSES)
		if !ok {
			logger.Fatal("Auth data type miss-match in %s", c.Request.URL.Path)
			c.AbortWithStatus(500)
			return
		}

		code, rsp, e := handler(c, ses)
		if e != nil {
			logger.Error("Error: %+v", e)
			c.AbortWithStatus(500)
			return
		}

		processResp(c, code, rsp)
	}
}

func AuthQN[TSES interface{}](cfg *AuthenticatorConfigure, handler func(*gin.Context, TSES, map[string]string) (int, interface{}, error), privilege string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ses_obj, queries, code := doAuthN(c, cfg, privilege)
		if code != 0 {
			c.AbortWithStatus(code)
			return
		}

		ses, ok := ses_obj.(TSES)
		if !ok {
			logger.Fatal("Auth data type miss-match in %s", c.Request.URL.Path)
			c.AbortWithStatus(500)
			return
		}

		code, rsp, e := handler(c, ses, queries)
		if e != nil {
			logger.Error("Error: %+v", e)
			c.AbortWithStatus(500)
			return
		}

		processResp(c, code, rsp)
	}
}
