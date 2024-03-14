package websvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/acsl-go/logger"
	"github.com/gin-gonic/gin"
)

type Response struct {
	ContentType string
	Body        []byte
}

type AuthenticatorConfigure struct {
	QueryToken     func(context.Context, string) (interface{}, string, error)
	CheckPrivilege func(context.Context, string, interface{}, string) error
	RefreshToken   func(context.Context, string, interface{}) error
	TsTolerance    int64
}

var (
	ErrNoPrivilege = errors.New("fobidden")
)

func parseQuery(c *gin.Context) (map[string]string, string, error) {
	queries := make(map[string]string)
	if e := c.BindQuery(&queries); e != nil {
		logger.Error("Auth: parse query failed", e)
		return nil, "", e
	}
	query_str := ""

	keys := make([]string, 0, len(queries))
	for k := range queries {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		query_str += k + "=" + queries[k] + "&"
	}
	if len(query_str) > 0 && query_str[len(query_str)-1] == '&' {
		query_str = query_str[:len(query_str)-1]
	}

	return queries, query_str, nil
}

func doAuth(c *gin.Context, cfg *AuthenticatorConfigure, privilege string) (interface{}, map[string]string, []byte, int) {
	auth_str := c.GetHeader("Authorization")
	if auth_str == "" {
		return nil, nil, nil, 401
	}

	auth_parts := strings.Split(strings.ReplaceAll(auth_str, "Bearer ", ""), ":")
	if len(auth_parts) != 3 {
		return nil, nil, nil, 401
	}

	ses_id := auth_parts[0]
	signature := auth_parts[1]
	timestamp, e := strconv.ParseInt(auth_parts[2], 10, 64)
	if e != nil {
		return nil, nil, nil, 401
	}

	ts_error := time.Now().UnixMilli() - timestamp
	if ts_error > cfg.TsTolerance || ts_error < -cfg.TsTolerance {
		return nil, nil, nil, 400
	}

	ses, token, e := cfg.QueryToken(c, ses_id)
	if e != nil {
		logger.Error("QueryToken Error: %+v", e)
		return nil, nil, nil, 500
	}

	if ses == nil || token == "" {
		return nil, nil, nil, 401
	}

	queries, query, e := parseQuery(c)
	if e != nil {
		return nil, nil, nil, 400
	}

	var body_bytes []byte = nil
	var body_hash string = ""

	if c.Request.ContentLength > 0 {
		body_bytes, e = io.ReadAll(c.Request.Body)
		if e != nil {
			logger.Error("Read body failed: %+v", e)
			return nil, nil, nil, 500
		}
		body_hash_bytes := sha256.Sum256(body_bytes)
		body_hash = hex.EncodeToString(body_hash_bytes[:])
	}

	sig_str := auth_parts[0] + ":" + token + ":" + auth_parts[2] + ":" + query + ":" + body_hash
	sig_hash := sha256.Sum256([]byte(sig_str))
	if signature != hex.EncodeToString(sig_hash[:]) {
		return nil, nil, nil, 401
	}

	if cfg.CheckPrivilege != nil && privilege != "" {
		if e := cfg.CheckPrivilege(c, ses_id, ses, privilege); e != nil {
			if errors.Is(e, ErrNoPrivilege) {
				return nil, nil, nil, 403
			} else {
				logger.Fatal("privilege checking failed: %+v", e)
				return nil, nil, nil, 500
			}
		}
	}

	if cfg.RefreshToken != nil {
		if e := cfg.RefreshToken(c, ses_id, ses); e != nil {
			logger.Fatal("token refresh failed: %+v", e)
			return nil, nil, nil, 500
		}
	}

	return ses, queries, body_bytes, 0
}

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

func Auth[TSES interface{}](cfg *AuthenticatorConfigure, handler func(*gin.Context, TSES) (int, interface{}, error), privilege string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ses_obj, _, _, code := doAuth(c, cfg, privilege)
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

func AuthD[TSES interface{}, TDATA interface{}](cfg *AuthenticatorConfigure, handler func(*gin.Context, TSES, TDATA) (int, interface{}, error), privilege string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ses_obj, _, data_bytes, code := doAuth(c, cfg, privilege)
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

		var data TDATA
		if e := json.Unmarshal(data_bytes, &data); e != nil {
			logger.Error("Error: %+v", e)
			c.AbortWithStatus(400)
			return
		}

		code, rsp, e := handler(c, ses, data)
		if e != nil {
			logger.Error("Error: %+v", e)
			c.AbortWithStatus(500)
			return
		}

		processResp(c, code, rsp)
	}
}

func AuthQ[TSES interface{}](cfg *AuthenticatorConfigure, handler func(*gin.Context, TSES, map[string]string) (int, interface{}, error), privilege string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ses_obj, queries, _, code := doAuth(c, cfg, privilege)
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

func AuthQD[TSES interface{}, TDATA interface{}](cfg *AuthenticatorConfigure, handler func(*gin.Context, TSES, map[string]string, TDATA) (int, interface{}, error), privilege string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ses_obj, queries, data_bytes, code := doAuth(c, cfg, privilege)
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

		var data TDATA
		if e := json.Unmarshal(data_bytes, &data); e != nil {
			logger.Error("Error: %+v", e)
			c.AbortWithStatus(400)
			return
		}

		code, rsp, e := handler(c, ses, queries, data)
		if e != nil {
			logger.Error("Error: %+v", e)
			c.AbortWithStatus(500)
			return
		}

		processResp(c, code, rsp)
	}
}
