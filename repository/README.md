# Repository Package

Generic, reusable repository patterns for database operations with optional caching support.

## Overview

The repository package provides interface-based design with concrete implementations for SQL databases. It uses Go generics for type safety and supports flexible filtering, pagination, and sorting.

## Features

- **Type-safe generic repositories** using Go generics
- **SQL repository implementation** with leader/follower support via sqlkit
- **Transaction support** through context-based transaction injection
- **Flexible filtering and pagination** with offset-based and cursor-based options
- **Mock repository** for testing without database
- **Extensible design** for custom repository implementations

## Package Structure

```
repository/
├── interface.go       # Core repository interfaces
├── options.go         # Common options (Filter, Pagination, Sort, PagedResult)
├── errors.go          # Repository-specific errors
├── sql/
│   ├── base.go        # Base repository with common logic
│   ├── generic.go     # Generic CRUD repository implementation
│   └── helpers.go     # SQL helper functions
├── cache/
│   ├── decorator.go   # TODO: Caching decorator (requires redis package)
│   ├── strategy.go    # TODO: Cache strategies
│   └── key.go         # TODO: Cache key generation
└── mock/
    └── repository.go  # Mock implementations for testing
```

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "database/sql"
    
    "github.com/biairmal/go-sdk/repository"
    "github.com/biairmal/go-sdk/repository/sql"
    "github.com/biairmal/go-sdk/sqlkit"
)

// Define entity
type User struct {
    ID        int64     `db:"id"`
    Name      string    `db:"name"`
    Email     string    `db:"email"`
    CreatedAt time.Time `db:"created_at"`
}

// Create repository
func NewUserRepository(db *sqlkit.DB) repository.Repository[User] {
    scanFunc := func(rows *sql.Rows) (*User, error) {
        var user User
        err := rows.Scan(&user.ID, &user.Name, &user.Email, &user.CreatedAt)
        return &user, err
    }
    
    return sql.NewGenericRepository[User](
        db,
        "users",
        scanFunc,
    )
}

// Use in service
func (s *UserService) CreateUser(ctx context.Context, name, email string) (*User, error) {
    user := &User{
        Name:  name,
        Email: email,
    }
    
    err := s.userRepo.Create(ctx, user)
    if err != nil {
        return nil, err
    }
    
    return user, nil
}
```

### List with Filters and Pagination

```go
func (s *UserService) GetActiveUsers(ctx context.Context, page int) ([]*User, error) {
    opts := &repository.ListOptions{
        Filter: repository.Filter{
            Conditions: map[string]any{
                "status": "active",
            },
        },
        Pagination: repository.Pagination{
            Limit:  20,
            Offset: page * 20,
        },
        Sort: repository.Sort{
            Field:     "created_at",
            Direction: repository.SortDesc,
        },
    }
    
    return s.userRepo.List(ctx, opts)
}
```

### List without Options (nil)

You can pass `nil` to `List` to retrieve all entities without filtering, pagination, or sorting:

```go
// Get all users without any options
users, err := s.userRepo.List(ctx, nil)
```

### With Transactions

```go
func (s *UserService) CreateUserWithWallet(ctx context.Context, req CreateUserRequest) error {
    return s.db.WithTransaction(ctx, func(txCtx context.Context) error {
        // Both repositories automatically use the transaction from txCtx
        
        user := &User{Name: req.Name, Email: req.Email}
        if err := s.userRepo.Create(txCtx, user); err != nil {
            return err
        }
        
        wallet := &Wallet{UserID: user.ID, Balance: 0}
        if err := s.walletRepo.Create(txCtx, wallet); err != nil {
            return err
        }
        
        return nil
    })
}
```

## Core Interfaces

### Repository[T]

Generic repository interface for CRUD operations:

```go
type Repository[T any] interface {
    Create(ctx context.Context, entity *T) error
    GetByID(ctx context.Context, id any) (*T, error)
    Update(ctx context.Context, id any, entity *T) error
    Delete(ctx context.Context, id any) error
    List(ctx context.Context, opts *ListOptions) ([]*T, error) // opts can be nil
    Count(ctx context.Context, filter Filter) (int64, error)
    Exists(ctx context.Context, id any) (bool, error)
}
```

**Note:** The `opts` parameter in `List` is a pointer and can be `nil`. When `nil`, the repository will return all entities without filtering, pagination, or sorting.

### ReadRepository[T] and WriteRepository[T]

Specialised interfaces for read-only and write-only operations.

## Options

### ListOptions

`ListOptions` is a pointer type, allowing it to be `nil` for convenience. When `nil`, the repository will return all entities without any filtering, pagination, or sorting.

```go
type ListOptions struct {
    Filter     Filter     // Filtering criteria
    Pagination Pagination // Pagination settings
    Sort       Sort       // Sorting settings
}
```

### Filter

Provides filtering capabilities:

```go
filter := repository.Filter{
    Conditions: map[string]any{
        "status": "active",
        "type":   "premium",
    },
    RawWhere: "created_at > ?",
    RawArgs:  []any{time.Now().AddDate(0, -1, 0)},
}
```

### Pagination

Supports both offset-based and cursor-based pagination:

```go
pagination := repository.Pagination{
    Limit:  20,
    Offset: 0,
    Cursor: "", // Optional cursor for cursor-based pagination
}
```

### Sort

Sorting options:

```go
sort := repository.Sort{
    Field:     "created_at",
    Direction: repository.SortDesc,
}
```

### PagedResult

Result wrapper for paginated queries:

```go
type PagedResult[T any] struct {
    Items      []*T   // Retrieved items
    Total      int64  // Total number of items
    Limit      int    // Items per page
    Offset     int    // Current offset
    Page       int    // Current page number (1-based)
    TotalPages int    // Total number of pages
    HasPrev    bool   // Whether there are previous pages
    HasNext    bool   // Whether there are more pages
    NextCursor string // Next page cursor
}
```

## Error Handling

The package provides standardised error types:

- `ErrNotFound` - Entity not found
- `ErrAlreadyExists` - Entity already exists
- `ErrInvalidID` - Invalid ID format
- `ErrInvalidEntity` - Entity validation failed
- `ErrConflict` - Update conflict
- `ErrConnection` - Database connection error

Use helper functions to check errors:

```go
if repository.IsNotFound(err) {
    // Handle not found
}
```

## Testing

Use the mock repository for unit testing:

```go
func TestUserRepository_Create(t *testing.T) {
    repo := mock.NewMockRepository[User]()
    
    user := &User{Name: "John", Email: "john@example.com"}
    err := repo.Create(context.Background(), user)
    
    assert.NoError(t, err)
}
```

## Security Considerations

1. **SQL Injection**: Always use parameterised queries (handled automatically)
2. **Column Name Validation**: Column names are sanitised, but maintain a whitelist for production
3. **Authorization**: Repository focuses on data access, not authorization - handle in service layer

## Limitations

- GenericRepository requires manual implementation of `ScanFunc` and query builders
- Caching decorator is not yet implemented (requires redis package)
- Complex queries may require custom repository implementations

## See Also

- [SQLKit Package](../sqlkit/README.md) - Database connection management
- [Repository Specification](../../specs/REPOSITORY_SPEC.md) - Detailed specification
