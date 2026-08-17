# Testing

Use `pkg/testing` helpers.

```go
c, rec := testing.NewTestContext("POST", "/api/v1/users", body)
handler(c)
testing.AssertStatus(t, rec, 200)
testing.AssertJSON(t, rec, map[string]any{"success": true})
```

Integration tests live in `tests/`.

```
go test ./...
```
