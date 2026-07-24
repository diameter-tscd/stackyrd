package pagination

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Cursor represents a pagination cursor
type Cursor struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Value     string    `json:"value,omitempty"`
}

// CursorPagination represents cursor-based pagination parameters
type CursorPagination struct {
	First  int     `json:"first,omitempty"`
	After  *Cursor `json:"after,omitempty"`
	Last   int     `json:"last,omitempty"`
	Before *Cursor `json:"before,omitempty"`
}

// CursorPage represents a page of results with cursor information
type CursorPage struct {
	Edges      []Edge   `json:"edges"`
	PageInfo   PageInfo `json:"page_info"`
	TotalCount int      `json:"total_count"`
}

// Edge represents an edge in a cursor-based pagination
type Edge struct {
	Node   interface{} `json:"node"`
	Cursor string      `json:"cursor"`
}

// PageInfo represents pagination metadata
type PageInfo struct {
	HasNextPage     bool    `json:"has_next_page"`
	HasPreviousPage bool    `json:"has_previous_page"`
	StartCursor     *string `json:"start_cursor,omitempty"`
	EndCursor       *string `json:"end_cursor,omitempty"`
}

// NewCursorPagination creates a new cursor pagination from query parameters
func NewCursorPagination(first, last int, after, before string) (*CursorPagination, error) {
	pagination := &CursorPagination{
		First: first,
		Last:  last,
	}

	if after != "" {
		cursor, err := DecodeCursor(after)
		if err != nil {
			return nil, fmt.Errorf("invalid after cursor: %w", err)
		}
		pagination.After = cursor
	}

	if before != "" {
		cursor, err := DecodeCursor(before)
		if err != nil {
			return nil, fmt.Errorf("invalid before cursor: %w", err)
		}
		pagination.Before = cursor
	}

	if pagination.First < 0 {
		pagination.First = 0
	}
	if pagination.Last < 0 {
		pagination.Last = 0
	}

	if pagination.First == 0 && pagination.Last == 0 {
		pagination.First = 10
	}

	return pagination, nil
}

// EncodeCursor encodes a cursor to a base64 string
func EncodeCursor(cursor *Cursor) (string, error) {
	data, err := json.Marshal(cursor)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// DecodeCursor decodes a base64 cursor string
func DecodeCursor(cursorStr string) (*Cursor, error) {
	data, err := base64.StdEncoding.DecodeString(cursorStr)
	if err != nil {
		return nil, err
	}

	var cursor Cursor
	if err := json.Unmarshal(data, &cursor); err != nil {
		return nil, err
	}

	return &cursor, nil
}

// OffsetPagination represents offset-based pagination parameters
type OffsetPagination struct {
	Page    int `json:"page,omitempty"`
	PerPage int `json:"per_page,omitempty"`
}

// NewOffsetPagination creates offset pagination from query params
func NewOffsetPagination(page, perPage int) *OffsetPagination {
	if page < 1 {
		page = 1
	}
	if perPage < 1 {
		perPage = 10
	}
	if perPage > 100 {
		perPage = 100
	}
	return &OffsetPagination{Page: page, PerPage: perPage}
}

// Offset returns the database offset (0-indexed)
func (p *OffsetPagination) Offset() int {
	return (p.Page - 1) * p.PerPage
}

// Limit returns the limit for SQL queries
func (p *OffsetPagination) Limit() int {
	return p.PerPage
}

// CursorPageBuilder helps build cursor-based paginated responses
type CursorPageBuilder struct {
	edges      []interface{}
	total      int
	cursorKey  string
	idKey      string
	timestampKey string
}

// NewCursorPageBuilder creates a new builder
func NewCursorPageBuilder(cursorKey, idKey, timestampKey string) *CursorPageBuilder {
	return &CursorPageBuilder{
		edges:      make([]Edge, 0),
		cursorKey:  cursorKey,
		idKey:      idKey,
		timestampKey: timestampKey,
	}
}

// AddItem adds an item to the page
func (b *CursorPageBuilder) AddItem(node interface{}, id string, timestamp time.Time) error {
	cursor := Cursor{ID: id, Timestamp: timestamp}
	cursorStr, err := EncodeCursor(&cursor)
	if err != nil {
		return err
	}
	b.edges = append(b.edges, Edge{Node: node, Cursor: cursorStr})
	return nil
}

// SetTotal sets the total count
func (b *CursorPageBuilder) SetTotal(total int) {
	b.total = total
}

// Build creates the final CursorPage
func (b *CursorPageBuilder) Build(hasNext, hasPrev bool) CursorPage {
	startCursor, endCursor := "", ""
	if len(b.edges) > 0 {
		startCursor = b.edges[0].Cursor
		endCursor = b.edges[len(b.edges)-1].Cursor
	}

	return CursorPage{
		Edges:      b.edges,
		PageInfo: PageInfo{
			HasNextPage:     hasNext,
			HasPreviousPage: hasPrev,
			StartCursor:     ptrString(startCursor),
			EndCursor:       ptrString(endCursor),
		},
		TotalCount: b.total,
	}
}

func ptrString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// ValidateCursor validates a cursor string format
func ValidateCursor(cursorStr string) error {
	if cursorStr == "" {
		return nil
	}
	_, err := DecodeCursor(cursorStr)
	return err
}

// CursorToOffset converts a cursor to an offset for hybrid pagination
func CursorToOffset(cursorStr string, limit int) (int, error) {
	if cursorStr == "" {
		return 0, nil
	}
	cursor, err := DecodeCursor(cursorStr)
	if err != nil {
		return 0, err
	}
	_ = cursor // Use cursor timestamp/ID for offset calculation in real impl
	return 0, nil
}

// CalculateTotalPages calculates total pages for pagination meta
func CalculateTotalPages(total, perPage int) int {
	if perPage <= 0 {
		return 0
	}
	pages := total / perPage
	if total%perPage > 0 {
		pages++
	}
	return pages
}

// ParsePaginationFromQuery parses pagination from echo query params
func ParsePaginationFromQuery(c interface {
	QueryParam(key string) string
}) (page, perPage int) {
	page = 1
	perPage = 10

	if p := c.QueryParam("page"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val > 0 {
			page = val
		}
	}

	if p := c.QueryParam("per_page"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val > 0 && val <= 100 {
			perPage = val
		}
	}

	return page, perPage
}