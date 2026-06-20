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

// CreateCursor creates a cursor from an item
func CreateCursor(id string, timestamp time.Time, value string) (*Cursor, error) {
	return &Cursor{
		ID:        id,
		Timestamp: timestamp,
		Value:     value,
	}, nil
}

// CreateEdge creates an edge from a node and cursor
func CreateEdge(node interface{}, cursor *Cursor) (*Edge, error) {
	cursorStr, err := EncodeCursor(cursor)
	if err != nil {
		return nil, err
	}

	return &Edge{
		Node:   node,
		Cursor: cursorStr,
	}, nil
}

// CreatePage creates a cursor page from edges
func CreatePage(edges []Edge, hasNextPage, hasPreviousPage bool, totalCount int) *CursorPage {
	page := &CursorPage{
		Edges:      edges,
		TotalCount: totalCount,
		PageInfo: PageInfo{
			HasNextPage:     hasNextPage,
			HasPreviousPage: hasPreviousPage,
		},
	}

	if len(edges) > 0 {
		startCursor := edges[0].Cursor
		endCursor := edges[len(edges)-1].Cursor
		page.PageInfo.StartCursor = &startCursor
		page.PageInfo.EndCursor = &endCursor
	}

	return page
}

// GetLimit returns the limit for the query
func (p *CursorPagination) GetLimit() int {
	if p.First > 0 {
		return p.First + 1
	}
	if p.Last > 0 {
		return p.Last + 1
	}
	return 11
}

// GetOffset returns the offset for the query (not used in cursor pagination)
func (p *CursorPagination) GetOffset() int {
	return 0
}

// HasAfterCursor returns true if there's an after cursor
func (p *CursorPagination) HasAfterCursor() bool {
	return p.After != nil
}

// HasBeforeCursor returns true if there's a before cursor
func (p *CursorPagination) HasBeforeCursor() bool {
	return p.Before != nil
}

// IsForwardPagination returns true if forward pagination is requested
func (p *CursorPagination) IsForwardPagination() bool {
	return p.First > 0 || (p.First == 0 && p.Last == 0)
}

// IsBackwardPagination returns true if backward pagination is requested
func (p *CursorPagination) IsBackwardPagination() bool {
	return p.Last > 0
}

// GetAfterID returns the after cursor ID
func (p *CursorPagination) GetAfterID() string {
	if p.After != nil {
		return p.After.ID
	}
	return ""
}

// GetBeforeID returns the before cursor ID
func (p *CursorPagination) GetBeforeID() string {
	if p.Before != nil {
		return p.Before.ID
	}
	return ""
}

// GetAfterTimestamp returns the after cursor timestamp
func (p *CursorPagination) GetAfterTimestamp() time.Time {
	if p.After != nil {
		return p.After.Timestamp
	}
	return time.Time{}
}

// GetBeforeTimestamp returns the before cursor timestamp
func (p *CursorPagination) GetBeforeTimestamp() time.Time {
	if p.Before != nil {
		return p.Before.Timestamp
	}
	return time.Time{}
}

// StringToInt converts a string to int
func StringToInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// IntToString converts an int to string
func IntToString(i int) string {
	return strconv.Itoa(i)
}
