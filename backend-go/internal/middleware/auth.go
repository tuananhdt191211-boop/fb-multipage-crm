package middleware

import (
	"net/http"
	"strings"

	"fb-multipage-crm/internal/models"
	"fb-multipage-crm/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func JWTAuth(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := ""
		authHeader := c.GetHeader("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		} else if q := c.Query("token"); q != "" {
			token = q
		}

		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "thiếu token xác thực"})
			return
		}

		claims, err := utils.ParseJWT(token, jwtSecret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("system_role", claims.SystemRole)
		c.Next()
	}
}

// RequireRoles kiểm tra role thực tế của user trong DB
func RequireRoles(db *gorm.DB, allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt64("user_id")
		var user models.User
		if err := db.Where("id = ? AND is_active = TRUE", userID).First(&user).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "tài khoản không hợp lệ hoặc đã bị khóa"})
			return
		}

		for _, role := range allowedRoles {
			if user.SystemRole == role {
				c.Set("current_user", user)
				c.Set("system_role", user.SystemRole)
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "bạn không có quyền truy cập chức năng này"})
	}
}
