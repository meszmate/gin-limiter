# gin-limiter
Golang Rate Limiter for Gin engineered for high-traffic applications, leveraging an atomic Redis Lua script to deliver single-round-trip, race-free rate limiting at any scale.
- Custom Error & Header handlers
- Single network round-trip
- Not faster CPU-wise: a plain Go implementation is ~2–6× faster and uses less RAM.
- Much faster network-wise: the Lua script bundles GET + INCR + SET + EXPIRE into one atomic round-trip.
- Race-free: the script is executed atomically by Redis; no WATCH/MULTI/EXEC dance.
- Lower latency: ~0.3–0.5 ms saved per request at 1 Gb/s, which matters under high RPS.

# Installation
```shell
go get github.com/meszmate/gin-limiter
```

# Example
```go
package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/meszmate/gin-limiter"
)

func main() {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	store := ratelimit.NewRedisStore(rdb, time.Minute, 100, false) // Maximum 100 requests in 1 minute

	opts := &ratelimit.Options{
		// rate-limit by API key or IP
		KeyFunc: func(c *gin.Context) string {
			key := c.GetHeader("X-API-Key")
			if key == "" {
				key = c.ClientIP()
			}
			return key
		},

		// Custom headers (Not required)
		BeforeHandler: func(c *gin.Context, info ratelimit.Info) {
			c.Header("X-RateLimit-Limit", fmt.Sprint(info.Limit))
			c.Header("X-RateLimit-Remaining", fmt.Sprint(info.RemainingHits))
			c.Header("X-RateLimit-Reset", fmt.Sprint(info.ResetTime.Unix()))
		},

		// Custom Errors (Not Required)
		ErrorHandler: func(c *gin.Context, info ratelimit.Info) {
			c.Header("Retry-After", fmt.Sprint(int(time.Until(info.ResetTime).Seconds())))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":   "rate limit exceeded",
				"resetAt": info.ResetTime.UTC().Format(time.RFC3339),
			})
		},
	}
    rl := ratelimit.Middleware(store, opts)

	r := gin.Default()

    // Apply for all endpoints
    r.Use(rl)

	r.GET("/profile", func(c *gin.Context) {
		c.JSON(200, gin.H{"user": "meszmate"})
	})

    // Apply for specific endpoint(s):
	// r.GET("/endpoint", rl, func(c *gin.Context) {
	// 	c.JSON(200, gin.H{"user": "meszmate"})
	// })

	r.Run(":8080")
}
```