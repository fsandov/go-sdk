package paginate

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestNewPagination_Basic(t *testing.T) {
	p, err := NewPagination(1, 10, 95)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Page != 1 || p.Limit != 10 {
		t.Errorf("page=%d limit=%d", p.Page, p.Limit)
	}
	if p.TotalItems != 95 {
		t.Errorf("expected total_items=95, got %d", p.TotalItems)
	}
	if p.TotalPages != 10 {
		t.Errorf("expected total_pages=10, got %d", p.TotalPages)
	}
	if !p.HasNext {
		t.Error("expected has_next=true")
	}
	if p.HasPrev {
		t.Error("expected has_prev=false for page 1")
	}
	if p.NextPage != 2 {
		t.Errorf("expected next_page=2, got %d", p.NextPage)
	}
	if p.PrevPage != 0 {
		t.Errorf("expected prev_page=0, got %d", p.PrevPage)
	}
}

func TestNewPagination_LastPage(t *testing.T) {
	p, _ := NewPagination(10, 10, 95)
	if p.HasNext {
		t.Error("expected has_next=false on last page")
	}
	if !p.HasPrev {
		t.Error("expected has_prev=true on page 10")
	}
	if p.PrevPage != 9 {
		t.Errorf("expected prev_page=9, got %d", p.PrevPage)
	}
}

func TestNewPagination_MiddlePage(t *testing.T) {
	p, _ := NewPagination(5, 10, 95)
	if !p.HasNext || !p.HasPrev {
		t.Error("expected both has_next and has_prev on middle page")
	}
	if p.NextPage != 6 || p.PrevPage != 4 {
		t.Errorf("next=%d prev=%d", p.NextPage, p.PrevPage)
	}
}

func TestNewPagination_ZeroTotalItems(t *testing.T) {
	p, _ := NewPagination(1, 10, 0)
	if p.TotalPages != 1 {
		t.Errorf("expected total_pages=1 for 0 items, got %d", p.TotalPages)
	}
	if p.HasNext {
		t.Error("expected has_next=false for 0 items")
	}
}

func TestNewPagination_InvalidPage(t *testing.T) {
	p, _ := NewPagination(-5, 10, 100)
	if p.Page != DefaultPage {
		t.Errorf("expected page to be normalized to %d, got %d", DefaultPage, p.Page)
	}
}

func TestNewPagination_InvalidLimit(t *testing.T) {
	p, _ := NewPagination(1, -1, 100)
	if p.Limit != DefaultLimit {
		t.Errorf("expected limit to be normalized to %d, got %d", DefaultLimit, p.Limit)
	}

	p2, _ := NewPagination(1, 9999, 100)
	if p2.Limit != DefaultLimit {
		t.Errorf("expected limit over max to be normalized to %d, got %d", DefaultLimit, p2.Limit)
	}
}

func TestGetOffset(t *testing.T) {
	tests := []struct {
		page, limit, expected int
	}{
		{1, 10, 0},
		{2, 10, 10},
		{3, 25, 50},
		{0, 10, 0},
		{-1, 10, 0},
		{1, 0, 0},
		{1, -5, 0},
	}
	for _, tt := range tests {
		got := GetOffset(tt.page, tt.limit)
		if got != tt.expected {
			t.Errorf("GetOffset(%d, %d) = %d, want %d", tt.page, tt.limit, got, tt.expected)
		}
	}
}

func TestValidateOptions(t *testing.T) {
	page, limit, err := ValidateOptions(0, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page != DefaultPage {
		t.Errorf("expected page normalized to %d, got %d", DefaultPage, page)
	}
	if limit != 10 {
		t.Errorf("expected limit=10, got %d", limit)
	}

	_, _, err = ValidateOptions(1, 0)
	if err != ErrInvalidLimit {
		t.Errorf("expected ErrInvalidLimit, got %v", err)
	}

	_, _, err = ValidateOptions(1, MaxLimit+1)
	if err != ErrInvalidLimit {
		t.Errorf("expected ErrInvalidLimit for over-max, got %v", err)
	}
}

func TestCalculateTotalPages(t *testing.T) {
	tests := []struct {
		total, perPage, expected int
	}{
		{100, 10, 10},
		{101, 10, 11},
		{0, 10, 0},
		{10, 0, 0},
		{10, -1, 0},
		{1, 1, 1},
		{99, 100, 1},
	}
	for _, tt := range tests {
		got := CalculateTotalPages(tt.total, tt.perPage)
		if got != tt.expected {
			t.Errorf("CalculateTotalPages(%d, %d) = %d, want %d", tt.total, tt.perPage, got, tt.expected)
		}
	}
}

func TestIsValidPage(t *testing.T) {
	tests := []struct {
		page, totalPages int
		expected         bool
	}{
		{1, 10, true},
		{10, 10, true},
		{11, 10, false},
		{0, 10, false},
		{-1, 5, false},
		{1, 0, true},
	}
	for _, tt := range tests {
		got := IsValidPage(tt.page, tt.totalPages)
		if got != tt.expected {
			t.Errorf("IsValidPage(%d, %d) = %v, want %v", tt.page, tt.totalPages, got, tt.expected)
		}
	}
}

func TestFromContext_Defaults(t *testing.T) {
	ctx := context.Background()
	page, limit, orderBy := FromContext(ctx)
	if page != DefaultPage || limit != DefaultLimit || orderBy != "" {
		t.Errorf("expected defaults, got page=%d limit=%d orderBy=%q", page, limit, orderBy)
	}
}

func TestGinPagination_ParsesQueryParams(t *testing.T) {
	e := gin.New()
	e.Use(GinPagination())

	var gotPage, gotLimit int
	var gotOrder string
	e.GET("/items", func(c *gin.Context) {
		gotPage, gotLimit, gotOrder = FromGinContext(c)
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/items?page=3&limit=25&order_by=name", nil)
	e.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotPage != 3 || gotLimit != 25 || gotOrder != "name" {
		t.Errorf("expected page=3 limit=25 order=name, got page=%d limit=%d order=%q", gotPage, gotLimit, gotOrder)
	}
}

func TestGinPagination_InvalidLimit(t *testing.T) {
	e := gin.New()
	e.Use(GinPagination())
	e.GET("/items", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/items?limit=5000", nil)
	e.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for over-max limit, got %d", w.Code)
	}
}
