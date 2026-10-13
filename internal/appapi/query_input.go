package appapi

import (
	"net/url"

	"git.hyhy.fun/rsplab/iolink/internal/domain"
	"github.com/gin-gonic/gin"
)

func parseQueryInput(c *gin.Context) (url.Values, error) {
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil {
		return nil, domain.ErrInvalidRange
	}
	return values, nil
}

func queryValueOrDefault(values url.Values, key, fallback string) string {
	if values.Has(key) {
		return values.Get(key)
	}
	return fallback
}
