package web

import (
	"strings"

	"github.com/fsandov/go-sdk/pkg/client"
	"github.com/gin-gonic/gin"
)

var UserIDContextKey = client.UserIDContextKey

var PermissionsContextKey = client.PermissionsContextKey

func ExtractUserID(c *gin.Context) string {
	return c.GetHeader("X-User-ID")
}

func ExtractUserPermissions(c *gin.Context) []string {
	header := c.GetHeader("X-User-Permissions")
	if header == "" {
		return nil
	}
	return strings.Split(header, ",")
}

func UserHasPermission(c *gin.Context, permission string) bool {
	for _, p := range ExtractUserPermissions(c) {
		if p == permission {
			return true
		}
	}
	return false
}
