# SQLKit Package

A driver-agnostic SQL database connection manager for Go with leader/follower (read/write split) support, health monitoring, and transaction management. Returns standard `*sql.DB` instances that work seamlessly with sqlc, sqlx, or raw SQL.

## Overview

SQLKit provides a clean abstraction for managing database connections in applications that require read/write splitting. It handles connection pooling, health monitoring, automatic failover, and context-based transaction management while maintaining compatibility with the standard `database/sql` package and popular Go SQL libraries.

The package is designed to be driver-agnostic—you import your preferred database driver (PostgreSQL, MySQL, SQLite, etc.), and SQLKit manages the connections. It returns standard `*sql.DB` instances, making it a drop-in replacement for direct `sql.Open()` usage while adding production-ready features like health checks, connection retry logic, and read/write splitting.

## Features

### Core Capabilities

- **Leader/Follower Architecture**: Separate read and write connections with automatic load balancing and failover
- **Round-Robin Load Balancing**: Distributes read queries across multiple follower databases
- **Health Monitoring**: Background health checks with configurable intervals and automatic unhealthy connection detection
- **Connection Pooling**: Configurable connection pool settings (max open/idle connections, lifetime, idle time)
- **Transaction Management**: Context-based transaction injection for seamless repository integration
- **Retry Logic**: Automatic connection retry with exponential backoff for transient failures
- **Driver Agnostic**: Works with any `database/sql` compatible driver (PostgreSQL, MySQL, SQLite, etc.)
- **Thread-Safe**: All operations are safe for concurrent use
- **Zero-Allocation Hot Paths**: Optimised for performance in high-throughput scenarios

### Use Cases

- **Read/Write Splitting**: Route read queries to replicas, writes to primary database
- **High Availability**: Automatic failover to leader when followers are unhealthy
- **Load Distribution**: Distribute read load across multiple database replicas
- **Health Monitoring**: Track database connection health for monitoring and alerting
- **Transaction Management**: Simplify transaction handling in service layers

## Limitations

### General Limitations

1. **No Query Builder**: SQLKit does not provide query building functionality. Use sqlc, sqlx, or squirrel for query building.

2. **No ORM Features**: SQLKit does not provide ORM capabilities. Use GORM or other ORMs if needed.

3. **No Migration Tools**: SQLKit does not include database migration tools. Use golang-migrate or similar tools.

4. **No Schema Management**: Schema management must be handled separately using appropriate tools.

5. **No Distributed Transactions**: SQLKit does not coordinate distributed transactions (2PC, Saga). This must be handled at the application layer.

6. **Single Database Type**: All leader and followers must use the same database driver (e.g., all PostgreSQL or all MySQL).

7. **Health Check Overhead**: Health checks run in a background goroutine and consume resources. Disable if not needed.

8. **Follower Selection**: Round-robin selection does not consider follower load or latency. It only checks health status.

9. **Nested Transactions**: Nested transactions are not supported. Calling `WithTransaction` or `WithTransactionOptions` from within an existing transaction returns an error.

## Usage

### Installation

```bash
go get github.com/biairmal/go-sdk/sqlkit
```

**Note**: You must also import your database driver:

```go
import (
    _ "github.com/lib/pq"        // PostgreSQL
    // or
    _ "github.com/go-sql-driver/mysql"  // MySQL
)
```

### Basic Setup

#### Simple Configuration (Leader Only)

```go
package main

import (
    "context"
    "log"
    "time"
    "github.com/biairmal/go-sdk/sqlkit"
)

func main() {
    ctx := context.Background()

    cfg := sqlkit.Config{
        Leader: sqlkit.DBConfig{
            Driver:         "postgres",
            Host:           "localhost",
            Port:           5432,
            Database:       "myapp",
            Username:       "app_user",
            Password:       "secret",
            SSLMode:        "require",
            ConnectTimeout: 5 * time.Second,
        },
    }

    db, err := sqlkit.New(ctx, &cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Use leader for both reads and writes
    _, err = db.Leader().ExecContext(ctx, "INSERT INTO users (name) VALUES ($1)", "John")
    if err != nil {
        log.Fatal(err)
    }
}
```

#### Full Configuration (Leader + Followers)

```go
cfg := sqlkit.Config{
    Leader: sqlkit.DBConfig{
        Driver:         "postgres",
        Host:           "db-leader.example.com",
        Port:           5432,
        Database:       "myapp",
        Username:       "app_user",
        Password:       "secret",
        SSLMode:        "require",
        ConnectTimeout: 5 * time.Second,
        MaxRetries:     3,
    },
    Followers: []sqlkit.DBConfig{
        {
            Driver:         "postgres",
            Host:           "db-replica-1.example.com",
            Port:           5432,
            Database:       "myapp",
            Username:       "readonly_user",
            Password:       "secret",
            SSLMode:        "require",
            ConnectTimeout: 5 * time.Second,
        },
        {
            Driver:         "postgres",
            Host:           "db-replica-2.example.com",
            Port:           5432,
            Database:       "myapp",
            Username:       "readonly_user",
            Password:       "secret",
            SSLMode:        "require",
            ConnectTimeout: 5 * time.Second,
        },
    },
    Pool: sqlkit.PoolConfig{
        MaxOpenConns:    25,
        MaxIdleConns:    5,
        ConnMaxLifetime: 5 * time.Minute,
        ConnMaxIdleTime: 1 * time.Minute,
    },
    Health: sqlkit.HealthConfig{
        Enabled:       true,
        CheckInterval: 30 * time.Second,
        Timeout:       5 * time.Second,
    },
}

db, err := sqlkit.New(ctx, &cfg)
if err != nil {
    log.Fatal(err)
}
defer db.Close()
```

### Read/Write Operations

#### Write Operations (Use Leader)

```go
// INSERT, UPDATE, DELETE operations
_, err := db.Leader().ExecContext(ctx,
    "INSERT INTO users (name, email) VALUES ($1, $2)",
    "John Doe", "john@example.com")

// Prepared statements
stmt, err := db.Leader().PrepareContext(ctx, "UPDATE users SET name = $1 WHERE id = $2")
defer stmt.Close()
_, err = stmt.ExecContext(ctx, "Jane Doe", 123)
```

#### Read Operations (Use Follower)

```go
// SELECT queries use follower (round-robin)
rows, err := db.Follower().QueryContext(ctx, "SELECT id, name, email FROM users")
if err != nil {
    return err
}
defer rows.Close()

for rows.Next() {
    var id int
    var name, email string
    if err := rows.Scan(&id, &name, &email); err != nil {
        return err
    }
    // Process row
}
```

### Integration with SQLC

SQLKit works seamlessly with [sqlc](https://sqlc.dev/):

```go
import (
    "github.com/biairmal/go-sdk/sqlkit"
    "your-app/db/sqlc"
)

// Create query instances
writeQueries := sqlc.New(db.Leader())
readQueries := sqlc.New(db.Follower())

// Write operation
user, err := writeQueries.CreateUser(ctx, sqlc.CreateUserParams{
    Name:  "John Doe",
    Email: "john@example.com",
})

// Read operation
users, err := readQueries.ListUsers(ctx)

// Transaction
err = db.WithTransaction(ctx, func(txCtx context.Context) error {
    qtx := writeQueries.WithTx(sqlkit.ExtractTx(txCtx))

    if err := qtx.CreateUser(txCtx, sqlc.CreateUserParams{...}); err != nil {
        return err
    }

    if err := qtx.CreateWallet(txCtx, sqlc.CreateWalletParams{...}); err != nil {
        return err
    }

    return nil
})
```

### Integration with SQLX

SQLKit works seamlessly with [sqlx](https://github.com/jmoiron/sqlx):

```go
import (
    "github.com/jmoiron/sqlx"
    "github.com/biairmal/go-sdk/sqlkit"
)

leaderDB := sqlx.NewDb(db.Leader(), db.Driver())
followerDB := sqlx.NewDb(db.Follower(), db.Driver())

// Write operation
_, err = leaderDB.ExecContext(ctx, "INSERT INTO users (name, email) VALUES ($1, $2)",
    "John Doe", "john@example.com")

// Read operation with struct scanning
var users []User
err = followerDB.SelectContext(ctx, &users, "SELECT * FROM users WHERE active = $1", true)

// Transaction
err = db.WithTransaction(ctx, func(txCtx context.Context) error {
    tx, _ := sqlkit.ExtractTx(txCtx)
    txx := sqlx.NewDb(tx, db.Driver())

    _, err := txx.ExecContext(txCtx, "INSERT INTO users...")
    return err
})
```

### Transaction Management

#### Basic Transaction

```go
err := db.WithTransaction(ctx, func(txCtx context.Context) error {
    // All repository calls use txCtx
    // They will automatically use the transaction

    if err := userRepo.Create(txCtx, user); err != nil {
        return err
    }

    if err := walletRepo.Create(txCtx, wallet); err != nil {
        return err
    }

    return nil // Commit transaction
    // Return error to rollback
})
```

#### Transaction with Custom Options

```go
opts := &sql.TxOptions{
    Isolation: sql.LevelSerializable,
    ReadOnly:  false,
}

err := db.WithTransactionOptions(ctx, opts, func(txCtx context.Context) error {
    // Transaction with custom isolation level
    return nil
})
```

#### Read-Only Transaction (on Follower)

```go
err := db.WithReadOnlyTransaction(ctx, func(txCtx context.Context) error {
    // Long-running read operations
    // Uses follower database
    // Provides consistent snapshot

    var report Report
    err := readQueries.GenerateReport(txCtx, &report)
    return err
})
```

### Repository Pattern with Transactions

Repositories that accept `context.Context` can participate in transactions by using `sqlkit.ExtractTx(ctx)`: when the service runs code inside `WithTransaction`, the same context is passed to the repository, which then uses the transaction for its queries.

```go
import (
    "context"
    "database/sql"
    "github.com/biairmal/go-sdk/sqlkit"
)

type UserRepository struct {
    db *sqlkit.DB
}

func (r *UserRepository) Create(ctx context.Context, user *User) error {
    conn := r.getConnection(ctx) // write connection
    _, err := conn.ExecContext(ctx, "INSERT INTO users (name) VALUES ($1)", user.Name)
    return err
}

func (r *UserRepository) getConnection(ctx context.Context) interface {
    ExecContext(context.Context, string, ...any) (sql.Result, error)
} {
    if tx, ok := sqlkit.ExtractTx(ctx); ok {
        return tx
    }
    return r.db.Leader()
}

// Service layer manages transactions
type UserService struct {
    db   *sqlkit.DB
    repo *UserRepository
}

func (s *UserService) CreateUserWithWallet(ctx context.Context, user *User, wallet *Wallet) error {
    return s.db.WithTransaction(ctx, func(txCtx context.Context) error {
        if err := s.repo.Create(txCtx, user); err != nil {
            return err
        }
        if err := s.walletRepo.Create(txCtx, wallet); err != nil {
            return err
        }
        return nil
    })
}
```

### Health Monitoring

#### Check Overall Health

```go
health := db.GetHealth()

fmt.Printf("Leader healthy: %v\n", health.Leader.Healthy)
fmt.Printf("Leader response time: %v\n", health.Leader.ResponseTime)
fmt.Printf("Last check: %v\n", health.Leader.LastCheck)
if health.Leader.Error != "" {
    fmt.Printf("Leader error: %s\n", health.Leader.Error)
}

for i, follower := range health.Followers {
    fmt.Printf("Follower %d healthy: %v\n", i, follower.Healthy)
}
```

#### Quick Health Check

```go
if db.IsHealthy() {
    // Leader is healthy
} else {
    // Leader is unhealthy
}
```

#### Health Check Endpoint

```go
func healthHandler(w http.ResponseWriter, r *http.Request) {
    health := db.GetHealth()

    if !health.Leader.Healthy {
        http.Error(w, "Database unhealthy", http.StatusServiceUnavailable)
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(health)
}
```

### Error Handling

```go
import (
    "database/sql"
    "github.com/biairmal/go-sdk/sqlkit"
)

func findUser(ctx context.Context, db *sqlkit.DB, id int) (*User, error) {
    var user User
    err := db.Follower().QueryRowContext(ctx,
        "SELECT id, name FROM users WHERE id = $1", id).
        Scan(&user.ID, &user.Name)

    if err != nil {
        if sqlkit.IsNoRows(err) {
            // Handle "not found" case
            return nil, errors.New("user not found")
        }
        // Handle other errors
        return nil, err
    }

    return &user, nil
}
```

## Configuration Reference

### Config Validation

`Config.Validate()` requires:

- `Leader.Driver` non-empty
- `Leader.Host` non-empty
- `Leader.Database` non-empty

Pool and Health defaults are applied in `New()` when zero values are present.

### DBConfig

Configuration for a single database connection.

```go
type DBConfig struct {
    Driver         string        // Database driver: "postgres", "mysql", "sqlite3"
    Host           string        // Database host
    Port           int           // Database port
    Database       string        // Database name
    Username       string        // Database username
    Password       string        // Database password
    SSLMode        string        // SSL mode: "disable", "require", "verify-ca", "verify-full" (postgres)
    ConnectTimeout time.Duration // Connection timeout (default: 5s)
    MaxRetries     int           // Maximum connection retry attempts (default: 3)
}
```

**DSN Generation**: The `DSN()` method generates database-specific connection strings:

- **PostgreSQL**: `host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d`
- **MySQL**: `%s:%s@tcp(%s:%d)/%s?parseTime=true&timeout=%s`
- **SQLite3**: `file:%s?mode=rwc&cache=shared&_busy_timeout=%d`

Passwords are automatically URL-encoded to handle special characters.

### PoolConfig

Connection pool configuration.

```go
type PoolConfig struct {
    MaxOpenConns    int           // Maximum open connections (default: 25)
    MaxIdleConns    int           // Maximum idle connections (default: 5)
    ConnMaxLifetime time.Duration // Maximum connection lifetime (default: 5m)
    ConnMaxIdleTime time.Duration // Maximum connection idle time (default: 1m)
}
```

Use `sqlkit.DefaultPoolConfig()` to get default values.

### HealthConfig

Health check configuration.

```go
type HealthConfig struct {
    Enabled       bool          // Enable health checks (default: true)
    CheckInterval time.Duration // Health check interval (default: 30s)
    Timeout       time.Duration // Health check timeout (default: 5s)
}
```

Use `sqlkit.DefaultHealthConfig()` to get default values.

## API Reference

### Functions

#### New

```go
func New(ctx context.Context, cfg *Config) (*DB, error)
```

Creates and initialises a new DB instance. Accepts a pointer to `Config`. Validates configuration, initialises leader and follower connections, configures connection pools, and starts health check goroutine if enabled. Returns error only if leader connection fails.

### Methods on DB

#### Leader

```go
func (db *DB) Leader() *sql.DB
```

Returns the leader (write) database connection. Thread-safe. Always returns non-nil if DB was successfully created. Use for write operations (INSERT, UPDATE, DELETE) and transactions that modify data.

#### Follower

```go
func (db *DB) Follower() *sql.DB
```

Returns a follower (read) database connection using round-robin load balancing. If no followers configured, returns leader. Checks follower health and falls back to leader if all followers are unhealthy. Thread-safe. Use for read operations (SELECT) and operations that can tolerate eventual consistency.

#### Driver

```go
func (db *DB) Driver() string
```

Returns the database driver name (e.g., "postgres", "mysql", "sqlite3").

#### Close

```go
func (db *DB) Close() error
```

Closes all database connections and stops health checks. Cancels context, closes leader and all follower connections, and collects any errors. Thread-safe.

#### WithTransaction

```go
func (db *DB) WithTransaction(ctx context.Context, fn TxFunc) error
```

Executes a function within a transaction with default options. Begins transaction on leader, injects transaction into context, executes function, and commits or rolls back based on result. Returns an error if called when a transaction is already present in the context (nested transaction). Panic-safe.

#### WithTransactionOptions

```go
func (db *DB) WithTransactionOptions(ctx context.Context, opts *sql.TxOptions, fn TxFunc) error
```

Same as `WithTransaction` but uses provided transaction options (isolation level, read-only flag). Nested transaction in context returns error.

#### WithReadOnlyTransaction

```go
func (db *DB) WithReadOnlyTransaction(ctx context.Context, fn TxFunc) error
```

Executes a read-only transaction on a follower. Uses follower database, falls back to leader if no healthy followers. Still requires commit. Nested transaction in context returns error.

#### GetHealth

```go
func (db *DB) GetHealth() Health
```

Returns current health status of all connections. Thread-safe.

#### IsHealthy

```go
func (db *DB) IsHealthy() bool
```

Returns true if leader is healthy. Thread-safe.

### Transaction Functions

#### InjectTx

```go
func InjectTx(ctx context.Context, tx *sql.Tx) context.Context
```

Injects a transaction into the context. Called internally by `WithTransaction`.

#### ExtractTx

```go
func ExtractTx(ctx context.Context) (*sql.Tx, bool)
```

Extracts a transaction from the context if present. Returns transaction and true if found, nil and false otherwise. Use in repositories to detect if they're in a transaction.

### Error Variables

```go
var (
    ErrNoConnection     = errors.New("sqlkit: no database connection")
    ErrLeaderUnhealthy  = errors.New("sqlkit: leader database unhealthy")
    ErrAllFollowersDown = errors.New("sqlkit: all follower databases down")
    ErrInvalidConfig    = errors.New("sqlkit: invalid configuration")
    ErrTransactionFailed = errors.New("sqlkit: transaction failed")
)
```

### Helper Functions

#### IsNoRows

```go
func IsNoRows(err error) bool
```

Checks if error is `sql.ErrNoRows`. Use in repository layer to distinguish "not found" from other errors.

## Migration Path

### From Raw database/sql

```go
// Before
db, err := sql.Open("postgres", dsn)

// After
sqlkitDB, err := sqlkit.New(ctx, &cfg)
db := sqlkitDB.Leader() // Drop-in replacement
```

### From SQLX

```go
// Before
db := sqlx.Connect("postgres", dsn)

// After
sqlkitDB, err := sqlkit.New(ctx, &cfg)
db := sqlx.NewDb(sqlkitDB.Leader(), "postgres")
```

## Performance Considerations

1. **Connection Pooling**: Use reasonable defaults (MaxOpenConns: 25, MaxIdleConns: 5). Monitor pool exhaustion in production.

2. **Health Checks**: Health checks run in a separate goroutine and do not block main operations. Default interval is 30 seconds. Disable if not needed.

3. **Follower Selection**: Round-robin selection is fast with minimal lock contention. Health checks are read-locked for quick lookups.

4. **Transaction Overhead**: Context value lookup is fast with zero allocation in hot paths. No reflection is used.

## Security Considerations

1. **Credentials**: Passwords are automatically URL-encoded in DSN generation. Never log passwords.

2. **SSL/TLS**: Always use SSL in production. Set `SSLMode` to "require" or "verify-full" for PostgreSQL.

3. **Connection Limits**: Configure reasonable connection pool limits to prevent connection exhaustion.

4. **Timeouts**: Always set `ConnectTimeout` to prevent hanging connections.

## Examples

### Complete Application Setup

```go
package main

import (
    "context"
    "log"
    "time"
    "github.com/biairmal/go-sdk/sqlkit"
    _ "github.com/lib/pq"
)

func main() {
    ctx := context.Background()

    cfg := sqlkit.Config{
        Leader: sqlkit.DBConfig{
            Driver:         "postgres",
            Host:           "localhost",
            Port:           5432,
            Database:       "myapp",
            Username:       "app_user",
            Password:       "secret",
            SSLMode:        "require",
            ConnectTimeout: 5 * time.Second,
        },
        Pool:   sqlkit.DefaultPoolConfig(),
        Health: sqlkit.DefaultHealthConfig(),
    }

    db, err := sqlkit.New(ctx, &cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Use database
    _, err = db.Leader().ExecContext(ctx, "INSERT INTO users (name) VALUES ($1)", "John")
    if err != nil {
        log.Fatal(err)
    }
}
```

### Service Layer with Transactions

```go
type UserService struct {
    db   *sqlkit.DB
    repo *UserRepository
}

func (s *UserService) CreateUserWithProfile(ctx context.Context, user *User, profile *Profile) error {
    return s.db.WithTransaction(ctx, func(txCtx context.Context) error {
        if err := s.repo.Create(txCtx, user); err != nil {
            return err
        }

        if err := s.profileRepo.Create(txCtx, profile); err != nil {
            return err
        }

        return nil
    })
}
```

## Dependencies

**Required:**

- `database/sql` (standard library)
- `context` (standard library)
- `sync` (standard library)
- `time` (standard library)
- `errors` (standard library)

**Database Drivers (user must import):**

- `github.com/lib/pq` (PostgreSQL)
- `github.com/go-sql-driver/mysql` (MySQL)
- Or any other `database/sql` compatible driver

**Optional:**

- `github.com/jmoiron/sqlx` (if user wants sqlx features)
- `github.com/kyleconroy/sqlc` (if user wants sqlc code generation)

## License

See the main repository license file.
