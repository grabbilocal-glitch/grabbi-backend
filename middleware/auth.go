package middleware

import (
	"net/http"
	"strings"

	"grabbi-backend/utils"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		token := parts[1]
		claims, err := utils.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("user_role", claims.Role)
		if claims.FranchiseID != nil {
			c.Set("franchise_id", *claims.FranchiseID)
		}
		c.Next()
	}
}

func AdminMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("user_role")
		if !exists || role != "admin" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// FranchiseMiddleware requires the user to be a franchise_owner or franchise_staff
// and have a franchise_id in their token.
func FranchiseMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("user_role")
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "Franchise access required"})
			c.Abort()
			return
		}

		roleStr := role.(string)
		if roleStr != "franchise_owner" && roleStr != "franchise_staff" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Franchise access required"})
			c.Abort()
			return
		}

		if _, exists := c.Get("franchise_id"); !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "No franchise associated with this account"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// FranchiseOwnerMiddleware requires the user to be specifically a franchise_owner.
func FranchiseOwnerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("user_role")
		if !exists || role != "franchise_owner" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Franchise owner access required"})
			c.Abort()
			return
		}
		c.Next()
	}
}
