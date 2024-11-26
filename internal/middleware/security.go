package middleware

import (
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type IPRateLimiter struct {
	ips map[string]*IPLimit
	mu  sync.RWMutex
}

type IPLimit struct {
	count    int
	lastSeen time.Time
}

func NewIPRateLimiter() *IPRateLimiter {
	return &IPRateLimiter{
		ips: make(map[string]*IPLimit),
	}
}

func (ipl *IPRateLimiter) RateLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()

		ipl.mu.Lock()
		limit, exists := ipl.ips[ip]

		now := time.Now()
		if !exists {
			ipl.ips[ip] = &IPLimit{count: 1, lastSeen: now}
			ipl.mu.Unlock()
			c.Next()
			return
		}

		// Reset counter if more than 1 minute has passed
		if now.Sub(limit.lastSeen) > time.Minute {
			limit.count = 0
			limit.lastSeen = now
		}

		limit.count++

		// Block if more than 60 requests per minute
		if limit.count > 60 {
			ipl.mu.Unlock()
			c.AbortWithStatus(http.StatusTooManyRequests)
			return
		}

		limit.lastSeen = now
		ipl.mu.Unlock()

		c.Next()
	}
}

func BlockSuspiciousRequests() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		// Block common attack patterns
		suspicious := []string{
			"eval-stdin.php",
			"phpunit",
			"../",
			"think\\app",
			".env",
			"wp-admin",
			"wp-login",
			"jenkins",
			"solr",
			"containers/json",
		}

		for _, pattern := range suspicious {
			if strings.Contains(path, pattern) || strings.Contains(query, pattern) {
				c.AbortWithStatus(404)
				return
			}
		}

		c.Next()
	}
}

func LogSuspiciousRequests() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if strings.Contains(path, "phpunit") ||
			strings.Contains(path, "eval-stdin.php") ||
			strings.Contains(path, "../") {

			// Log suspicious request
			log.Printf("[SECURITY] Suspicious request from IP %s to %s",
				c.ClientIP(), path)
		}
		c.Next()
	}
}
