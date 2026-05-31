package pagination

import (
	"strconv"
	"time"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

type Params struct {
	Limit  int
	Cursor *Cursor
}

type Cursor struct {
	CreatedAt time.Time
	ID        string
}

type PageInfo struct {
	HasNextPage bool    `json:"has_next_page"`
	NextCursor  *string `json:"next_cursor,omitempty"`
	Total       int     `json:"total,omitempty"`
}

func ParseParams(limitStr, cursorStr string) Params {
	limit := DefaultLimit
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		if l > MaxLimit {
			l = MaxLimit
		}
		limit = l
	}

	p := Params{Limit: limit}

	return p
}

func ParsePage(pageStr, limitStr string) (int, int) {
	page := 1
	limit := DefaultLimit

	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		if l > MaxLimit {
			l = MaxLimit
		}
		limit = l
	}

	return page, limit
}

func Offset(page, limit int) int {
	if page < 1 {
		page = 1
	}
	return (page - 1) * limit
}

type Meta struct {
	Page       int  `json:"page"`
	Limit      int  `json:"limit"`
	Total      int  `json:"total,omitempty"`
	HasNextPage bool `json:"has_next_page"`
}

func NewMeta(page, limit, count int) Meta {
	return Meta{
		Page:        page,
		Limit:       limit,
		Total:       count,
		HasNextPage: count == limit,
	}
}

// QueryParams holds parsed page/limit/offset for handler convenience.
type QueryParams struct {
	Page   int
	Limit  int
	Offset int
}

// FromFiber parses page and limit from fiber context query params.
func FromFiber(pageStr, limitStr string) QueryParams {
	page, limit := ParsePage(pageStr, limitStr)
	return QueryParams{Page: page, Limit: limit, Offset: Offset(page, limit)}
}

// MetaFromTotal builds a Meta response from total count and query params.
func MetaFromTotal(total int, qp QueryParams) Meta {
	return Meta{
		Page:        qp.Page,
		Limit:       qp.Limit,
		Total:       total,
		HasNextPage: qp.Offset+qp.Limit < total,
	}
}
