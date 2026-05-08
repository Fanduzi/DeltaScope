# Performance Guide

## Benchmarks

### Running Benchmarks

```bash
# Run all benchmarks
go test -bench=. -benchmem ./internal/application/audit/

# Run specific benchmark
go test -bench=BenchmarkAuditSimpleSQL -benchmem ./internal/application/audit/

# Run with profiling
go test -bench=BenchmarkAuditSimpleSQL -cpuprofile=cpu.prof -memprofile=mem.prof ./internal/application/audit/

# Analyze profile
go tool pprof cpu.prof
go tool pprof mem.prof
```

### Benchmark Results

Performance as of 2026-05-08 (Apple M4 Pro, darwin/arm64):

| Benchmark | Time (ns/op) | Memory (B/op) | Allocs |
|-----------|-------------|---------------|--------|
| AuditSimpleSQL | ~101,000 | ~198,000 | ~1,132 |
| AuditComplexSQL | ~191,000 | ~271,000 | ~1,665 |
| AuditMultiStatement (10 tables) | ~426,000 | ~625,000 | ~3,847 |
| AuditSQLTiDB | ~191,000 | ~271,000 | ~1,665 |

## Optimization Guidelines

### Slice Preallocation

Always preallocate slices when the size is known or can be estimated:

```go
// Good - registry accumulates findings from multiple rules
findings := make([]Finding, 0, 16)

// Bad - zero capacity triggers growth on first append
findings := make([]Finding, 0)
```

### String Building

Use `strings.Builder` with `Grow()` for string concatenation:

```go
// Good
builder := output.GetBuilder()
defer output.PutBuilder(builder)
builder.Grow(estimatedSize)
for _, item := range items {
    builder.WriteString(item)
}

// Bad
var result string
for _, item := range items {
    result += item
}
```

For integer formatting, prefer `strconv.Itoa` or `strconv.FormatInt` over `fmt.Sprintf`:

```go
// Good
builder.WriteString(strconv.Itoa(count))

// Bad
builder.WriteString(fmt.Sprintf("%d", count))
```

### Builder Reuse

Use the `sync.Pool`-backed builder pool for output rendering:

```go
// Good
builder := output.GetBuilder()
defer output.PutBuilder(builder)

// Bad
var builder strings.Builder
```

## Performance Targets

- Audit speed: <5ms for simple SQL
- Memory usage: <500KB per audit
- Allocations: <1,200 per simple audit
