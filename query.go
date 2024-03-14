package websvc

import (
	"slices"

	"github.com/acsl-go/logger"
	"github.com/gin-gonic/gin"
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
