package paginate

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	DefaultLimit = 10
	MaxLimit     = 1000
	DefaultPage  = 1
)

var (
	ErrInvalidLimit = errors.New("limit must be between 1 and 1000")
)

type ctxKeyPagination struct{}

type Pagination struct {
	Limit      int  `json:"limit"`
	Page       int  `json:"page"`
	TotalItems int  `json:"total_items"`
	TotalPages int  `json:"total_pages"`
	NextPage   int  `json:"next_page,omitempty"`
	PrevPage   int  `json:"prev_page,omitempty"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

type PaginatedResponse[T any] struct {
	Data       []T         `json:"data"`
	Pagination *Pagination `json:"pagination,omitempty"`
}

type Options struct {
	Page    int
	Limit   int
	OrderBy string
}

func NewPagination(page, limit, totalItems int) (*Pagination, error) {
	if page < 1 {
		page = DefaultPage
	}
	if limit < 1 || limit > MaxLimit {
		limit = DefaultLimit
	}
	totalPages := 1
	if totalItems > 0 {
		totalPages = int(math.Ceil(float64(totalItems) / float64(limit)))
	}
	hasNext := page < totalPages
	hasPrev := page > 1

	var nextPage, prevPage int
	if hasNext {
		nextPage = page + 1
	}
	if hasPrev {
		prevPage = page - 1
	}
	return &Pagination{
		Limit:      limit,
		Page:       page,
		TotalItems: totalItems,
		TotalPages: totalPages,
		NextPage:   nextPage,
		PrevPage:   prevPage,
		HasNext:    hasNext,
		HasPrev:    hasPrev,
	}, nil
}

func NewPaginatedResponse[T any](data []T, p *Pagination) *PaginatedResponse[T] {
	return &PaginatedResponse[T]{
		Data:       data,
		Pagination: p,
	}
}

func GetOffset(page, limit int) int {
	if page < 1 || limit < 1 {
		return 0
	}
	return (page - 1) * limit
}

func ValidateOptions(page, limit int) (int, int, error) {
	if page < 1 {
		page = DefaultPage
	}
	if limit < 1 || limit > MaxLimit {
		return 0, 0, ErrInvalidLimit
	}
	return page, limit, nil
}

func GinPagination() gin.HandlerFunc {
	return func(c *gin.Context) {
		page, _ := strconv.Atoi(c.DefaultQuery("page", strconv.Itoa(DefaultPage)))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(DefaultLimit)))
		orderBy := c.DefaultQuery("order_by", "")

		page, limit, err := ValidateOptions(page, limit)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		opts := &Options{
			Page:    page,
			Limit:   limit,
			OrderBy: orderBy,
		}

		c.Set("pagination", opts)

		ctx := context.WithValue(c.Request.Context(), ctxKeyPagination{}, opts)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func FromGinContext(c *gin.Context) (page, limit int, orderBy string) {
	if val, exists := c.Get("pagination"); exists {
		if p, ok := val.(*Options); ok && p != nil {
			return p.Page, p.Limit, p.OrderBy
		}
	}
	return DefaultPage, DefaultLimit, ""
}

func FromContext(ctx context.Context) (page, limit int, orderBy string) {
	if p, ok := ctx.Value(ctxKeyPagination{}).(*Options); ok && p != nil {
		return p.Page, p.Limit, p.OrderBy
	}
	return DefaultPage, DefaultLimit, ""
}

func ApplyGormPaginationFromContext(ctx context.Context, db *gorm.DB) *gorm.DB {
	page, limit, orderBy := FromContext(ctx)
	if limit == 0 {
		limit = DefaultLimit
	}
	if page == 0 {
		page = DefaultPage
	}
	db = db.Offset(GetOffset(page, limit)).Limit(limit)
	if orderBy != "" {
		db = db.Order(orderBy)
	}
	return db
}

func CalculateTotalPages(totalItems, itemsPerPage int) int {
	if itemsPerPage <= 0 {
		return 0
	}
	return int(math.Ceil(float64(totalItems) / float64(itemsPerPage)))
}

func IsValidPage(page, totalPages int) bool {
	return page >= 1 && (totalPages == 0 || page <= totalPages)
}
