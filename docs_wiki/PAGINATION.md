# Cursor-Based Pagination

Relay/GraphQL-style cursor pagination with base64-encoded cursors, forward/backward navigation.

## Overview

The `pkg/pagination/` package implements cursor-based pagination using opaque, base64-encoded cursors. Unlike offset/limit pagination, cursor pagination is stable under concurrent writes and provides consistent page boundaries.

## Concepts

```mermaid
flowchart LR
    subgraph Page1 [Page 1 (forward)]
        E1[Edge 1<br/>startCursor]
        E2[Edge 2]
        E3[Edge 3<br/>endCursor: cursor_3]
    end
    subgraph Page2 [Page 2 (forward)]
        E4[Edge 4<br/>startCursor]
        E5[Edge 5]
        E6[Edge 6<br/>endCursor: cursor_6]
    end
    Page1 -->|after: cursor_2| Page2
    Note1[hasNextPage: true] -.-> Page1
    Note2[hasNextPage: false] -.-> Page2
```

## Quick Start

### Request

```go
// Parse from query parameters
p, err := pagination.NewCursorPagination(
    first,      // page size for forward pagination
    last,       // page size for backward pagination
    after,      // cursor string from previous page (base64)
    before,     // cursor string for backward navigation
)
```

### Build Query

```go
limit := p.GetLimit() // over-fetches by 1 to detect next page

query := db.Model(&User{}).Limit(limit)

if p.IsForwardPagination() {
    if p.HasAfterCursor() {
        query = query.Where("id > ?", p.GetAfterID())
    }
    query = query.Order("id ASC")
} else {
    if p.HasBeforeCursor() {
        query = query.Where("id < ?", p.GetBeforeID())
    }
    query = query.Order("id DESC")
}
```

### Build Response

```go
// Create edges with encoded cursors
var edges []pagination.Edge
for _, user := range users {
    cursor := pagination.CreateCursor(user.ID, user.CreatedAt, "")
    edges = append(edges, pagination.CreateEdge(user, cursor))
}

// Create page with pagination info
hasNext := len(users) > first // over-fetched by 1
page := pagination.CreatePage(
    edges[:min(len(edges), first)],
    hasNext,
    false,
    totalCount,
)
```

## API Reference

### Core Types

```go
type Cursor struct {
    ID        string    `json:"id"`
    Timestamp time.Time `json:"timestamp"`
    Value     string    `json:"value,omitempty"`
}

type CursorPagination struct {
    First  int      // forward page size
    After  *Cursor  // cursor to fetch after
    Last   int      // backward page size
    Before *Cursor  // cursor to fetch before
}

type Edge struct {
    Node   interface{} `json:"node"`
    Cursor string      `json:"cursor"` // base64-encoded
}

type PageInfo struct {
    HasNextPage     bool   `json:"has_next_page"`
    HasPreviousPage bool   `json:"has_previous_page"`
    StartCursor     string `json:"start_cursor"`
    EndCursor       string `json:"end_cursor"`
}

type Page struct {
    Edges      []Edge   `json:"edges"`
    PageInfo   PageInfo `json:"page_info"`
    TotalCount int      `json:"total_count"`
}
```

### Functions

| Function | Description |
|----------|-------------|
| `NewCursorPagination(first, last int, after, before string)` | Parse query params into pagination struct |
| `EncodeCursor(cursor *Cursor) string` | Marshal + base64 encode |
| `DecodeCursor(cursorStr string) (*Cursor, error)` | Base64 decode + unmarshal |
| `CreateCursor(id string, ts time.Time, value string) *Cursor` | Build a cursor |
| `CreateEdge(node interface{}, cursor *Cursor) Edge` | Build an edge with encoded cursor |
| `CreatePage(edges []Edge, hasNext, hasPrev bool, total int) *Page` | Build a page with pagination info |

### Pagination Methods

| Method | Returns | Description |
|--------|---------|-------------|
| `GetLimit()` | int | Returns First+1 / Last+1 / 11 (over-fetch by 1) |
| `GetOffset()` | int | Always 0 |
| `IsForwardPagination()` | bool | True if First > 0 |
| `IsBackwardPagination()` | bool | True if Last > 0 |
| `HasAfterCursor()` | bool | After cursor is present |
| `HasBeforeCursor()` | bool | Before cursor is present |
| `GetAfterID()` | string | After cursor's ID |
| `GetBeforeID()` | string | Before cursor's ID |

## Integration with Services

```go
func (s *UserService) handleList(c echo.Context) error {
    // Parse pagination params
    first, _ := strconv.Atoi(c.QueryParam("first"))
    if first == 0 {
        first = 10
    }
    after := c.QueryParam("after")
    p, _ := pagination.NewCursorPagination(first, 0, after, "")

    // Query with over-fetch
    limit := p.GetLimit()
    var users []User
    tx := db.Model(&User{}).Limit(limit).Order("id ASC")
    if p.HasAfterCursor() {
        tx = tx.Where("id > ?", p.GetAfterID())
    }
    tx.Find(&users)

    // Count total
    var total int64
    db.Model(&User{}).Count(&total)

    // Build paginated response
    edges := make([]pagination.Edge, 0, len(users))
    for _, u := range users {
        cursor := pagination.CreateCursor(u.ID, u.CreatedAt, "")
        edges = append(edges, pagination.CreateEdge(u, cursor))
    }

    hasNext := len(users) > first
    if len(edges) > first {
        edges = edges[:first]
    }

    page := pagination.CreatePage(edges, hasNext, false, int(total))
    return response.SuccessWithMeta(c, page.Edges, map[string]interface{}{
        "page_info":     page.PageInfo,
        "total_count":   page.TotalCount,
    })
}
```

## Frontend Usage

### Request

```javascript
// First page
fetch('/api/v1/users?first=10')

// Next page
fetch('/api/v1/users?first=10&after=' + encodeURIComponent(pageInfo.endCursor))

// Previous page
fetch('/api/v1/users?last=10&before=' + encodeURIComponent(pageInfo.startCursor))
```

### Response

```json
{
  "success": true,
  "data": [
    { "node": { "id": "1", "name": "Alice" }, "cursor": "eyJpZCI6IjEifQ==" },
    { "node": { "id": "2", "name": "Bob" }, "cursor": "eyJpZCI6IjIifQ==" }
  ],
  "meta": {
    "page_info": {
      "has_next_page": true,
      "has_previous_page": false,
      "start_cursor": "eyJpZCI6IjEifQ==",
      "end_cursor": "eyJpZCI6IjIifQ=="
    },
    "total_count": 50
  }
}
```

## Comparison: Cursor vs Offset Pagination

| Aspect | Cursor | Offset |
|--------|--------|--------|
| Stability | Stable under writes | Skewed if rows added/removed |
| Performance | Efficient (index scan) | Degrades with high offset |
| Random access | No | Yes (page N) |
| UX | "Load more" pattern | Page numbers |
| Implementation | More complex | Simple |
