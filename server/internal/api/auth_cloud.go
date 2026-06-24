//go:build cloud

package api

import (
	"log"
	"net/http"
	"strings"

	"hn-reader/internal/storage"

	"github.com/gin-gonic/gin"
)

// authMiddleware validates the Bearer token and injects the user into context.
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		apiKey := strings.TrimPrefix(auth, "Bearer ")
		apiKey = strings.TrimSpace(apiKey)
		if apiKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing Authorization header"})
			return
		}
		user, err := s.repo.GetUserByAPIKey(apiKey)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if user == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
			return
		}
		c.Set("user", user)
		c.Next()
	}
}

// quotaMiddleware checks and deducts usage quota for summarization endpoints.
func (s *Server) quotaMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.HasPrefix(c.Request.URL.Path, "/api/summarize/") {
			c.Next()
			return
		}
		user := getUser(c)
		if user == nil {
			c.Next()
			return
		}
		quota, err := s.repo.GetUserQuota(user.ID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if quota.QuotaUsed >= quota.QuotaLimit {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "quota exceeded",
				"quota_used":  quota.QuotaUsed,
				"quota_limit": quota.QuotaLimit,
				"upgrade_url": "https://hn-reader.app/upgrade",
			})
			return
		}
		c.Set("quota_checked", true)
		c.Next()
	}
}

// chargeQuota deducts n from the current user's quota.
func (s *Server) chargeQuota(c *gin.Context, n int) {
	user := getUser(c)
	if user == nil {
		return
	}
	if !c.GetBool("quota_checked") {
		return
	}
	if err := s.repo.IncrementQuotaUsed(user.ID, n); err != nil {
		log.Printf("charge quota failed: %v", err)
	}
}

// getUser extracts the authenticated user from the Gin context.
func getUser(c *gin.Context) *storage.User {
	u, exists := c.Get("user")
	if !exists {
		return nil
	}
	user, ok := u.(*storage.User)
	if !ok {
		return nil
	}
	return user
}
