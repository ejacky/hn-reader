package api

import (
	"net/http"
	"strconv"
	"time"

	"hn-reader/internal/config"
	"hn-reader/internal/ratelimit"

	"github.com/gin-gonic/gin"
)

type limiters struct {
	signupHour *ratelimit.Limiter
	signupDay  *ratelimit.Limiter
	api        *ratelimit.Limiter
	summarize  *ratelimit.Limiter
	preload    *ratelimit.Limiter
	signal     *ratelimit.Limiter
	account    *ratelimit.Limiter
}

func newLimiters(cfg *config.Config) *limiters {
	return &limiters{
		signupHour: ratelimit.New(intOr(cfg.SignupRatePerHour, 5), time.Hour),
		signupDay:  ratelimit.New(intOr(cfg.SignupRatePerDay, 20), 24*time.Hour),
		api:        ratelimit.New(intOr(cfg.APIRatePerMin, 120), time.Minute),
		summarize:  ratelimit.New(intOr(cfg.SummarizeRatePerMin, 20), time.Minute),
		preload:    ratelimit.New(intOr(cfg.PreloadRatePerMin, 10), time.Minute),
		signal:     ratelimit.New(120, time.Minute),
		account:    ratelimit.New(5, time.Minute),
	}
}

func intOr(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

func (s *Server) signupLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.cloud {
			c.Next()
			return
		}
		ip := c.ClientIP()
		if ok, retry := s.limiters.signupHour.Allow(ip); !ok {
			abortRateLimit(c, retry)
			return
		}
		if ok, retry := s.limiters.signupDay.Allow(ip); !ok {
			abortRateLimit(c, retry)
			return
		}
		c.Next()
	}
}

func (s *Server) apiLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.cloud {
			c.Next()
			return
		}
		user := getUser(c)
		if user == nil {
			c.Next()
			return
		}
		if ok, retry := s.limiters.api.Allow(user.ID); !ok {
			abortRateLimit(c, retry)
			return
		}
		c.Next()
	}
}

func (s *Server) summarizeLimit() gin.HandlerFunc {
	return s.userLimit(s.limiters.summarize)
}

func (s *Server) preloadLimit() gin.HandlerFunc {
	return s.userLimit(s.limiters.preload)
}

func (s *Server) signalLimit() gin.HandlerFunc {
	return s.userLimit(s.limiters.signal)
}

func (s *Server) accountLimit() gin.HandlerFunc {
	return s.userLimit(s.limiters.account)
}

func (s *Server) userLimit(l *ratelimit.Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.cloud {
			c.Next()
			return
		}
		user := getUser(c)
		if user == nil {
			c.Next()
			return
		}
		if ok, retry := l.Allow(user.ID); !ok {
			abortRateLimit(c, retry)
			return
		}
		c.Next()
	}
}

func abortRateLimit(c *gin.Context, retry time.Duration) {
	secs := int(retry.Seconds())
	if secs < 1 {
		secs = 1
	}
	c.Header("Retry-After", strconv.Itoa(secs))
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
		"error":               "rate_limited",
		"retry_after_seconds": secs,
	})
}
