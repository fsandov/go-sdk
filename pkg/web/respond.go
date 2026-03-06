package web

import (
	"net/http"

	"github.com/fsandov/go-sdk/pkg/paginate"
	"github.com/gin-gonic/gin"
)

func JSON(c *gin.Context, status int, data any) {
	c.JSON(status, data)
}

func JSONSuccess(c *gin.Context, data any) {
	c.JSON(http.StatusOK, data)
}

func JSONCreated(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, data)
}

func JSONNoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func JSONError(c *gin.Context, status int, code, message string) {
	c.JSON(status, gin.H{
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}

func JSONPaginated[T any](c *gin.Context, data []T, pagination *paginate.Pagination) {
	c.JSON(http.StatusOK, paginate.PaginatedResponse[T]{
		Data:       data,
		Pagination: pagination,
	})
}
