//go:build cloud

package api

import (
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
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal_error"})
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
