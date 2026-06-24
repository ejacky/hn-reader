//go:build !cloud

package api

import (
	"hn-reader/internal/storage"

	"github.com/gin-gonic/gin"
)

// authMiddleware is a no-op for local mode.
func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// quotaMiddleware is a no-op for local mode.
func (s *Server) quotaMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

// chargeQuota is a no-op for local mode.
func (s *Server) chargeQuota(c *gin.Context, n int) {}

// getUser always returns nil in local mode (single user, no auth).
func getUser(c *gin.Context) *storage.User {
	return nil
}
