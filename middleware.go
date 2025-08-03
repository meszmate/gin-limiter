package ratelimit

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Options struct {
	KeyFunc       func(c *gin.Context) string
	ErrorHandler  func(c *gin.Context, info Info)
	BeforeHandler func(c *gin.Context, info Info)
}

func Middleware(s Store, opts *Options) gin.HandlerFunc {
	if opts == nil {
		opts = &Options{}
	}
	if opts.KeyFunc == nil {
		opts.KeyFunc = func(c *gin.Context) string {
			return c.ClientIP()
		}
	}
	if opts.ErrorHandler == nil {
		opts.ErrorHandler = func(c *gin.Context, info Info) {
			c.Header("X-Rate-Limit-Limit", fmt.Sprintf("%d", info.Limit))
			c.Header("X-Rate-Limit-Remaining", "0")
			c.Header("X-Rate-Limit-Reset", fmt.Sprintf("%d", info.ResetTime.Unix()))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "too many requests",
			})
		}
	}
	if opts.BeforeHandler == nil {
		opts.BeforeHandler = func(c *gin.Context, info Info) {
			c.Header("X-Rate-Limit-Limit", fmt.Sprintf("%d", info.Limit))
			c.Header("X-Rate-Limit-Remaining", fmt.Sprintf("%d", info.RemainingHits))
			c.Header("X-Rate-Limit-Reset", fmt.Sprintf("%d", info.ResetTime.Unix()))
		}
	}

	return func(c *gin.Context) {
		key := opts.KeyFunc(c)
		info := s.Limit(c.Request.Context(), key)
		opts.BeforeHandler(c, info)
		if info.RateLimited {
			opts.ErrorHandler(c, info)
			c.Abort()
			return
		}
		c.Next()
	}
}
