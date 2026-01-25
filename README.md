# srvr

A Go HTTP server library that provides dual-server architecture with built-in observability, health checks, and structured logging.

## Overview

`srvr` is designed to simplify the creation of production-ready HTTP servers by providing:

- **Dual Server Architecture**: Separate internal and external servers to protect internal endpoints
- **Built-in Observability**: Prometheus metrics, structured logging, and health checks
- **Flexible Configuration**: Functional options pattern for easy customization
- **Production Ready**: Proper timeouts, header limits, and error handling

## Installation

```bash
go get github.com/tpyle/srvr
```

## Quick Start

### Basic Server

```go
package main

import (
    "github.com/tpyle/srvr"
    "github.com/gorilla/mux"
)

func main() {
    // Create routers
    externalRouter := mux.NewRouter()
    externalRouter.HandleFunc("/api/hello", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("Hello, World!"))
    })

    // Create server with external router
    server := srvr.Create(
        srvr.WithExternalRouter(externalRouter),
    )

    // Start the server
    if err := server.Start(); err != nil {
        panic(err)
    }

    // Server is now running on:
    // - External API: http://localhost:8080
    // - Internal endpoints: http://localhost:8081
}
```

### With Custom Configuration

```go
server := srvr.Create(
    srvr.WithExternalPort(3000),
    srvr.WithInternalPort(3001),
    srvr.WithExternalRouter(externalRouter),
    srvr.WithInternalRouter(internalRouter),
    srvr.WithReadTimeout(30 * time.Second),
    srvr.WithWriteTimeout(30 * time.Second),
)
```

## Server Architecture

### Dual Server Design

The library creates two HTTP servers:

1. **External Server** (default port 8080): For your public API endpoints
2. **Internal Server** (default port 8081): For observability and management endpoints

### Default Internal Endpoints

The internal server automatically provides:

- `/live` - Liveness probe (always returns 200 OK)
- `/ready` - Readiness probe (runs health checks)
- `/metrics` - Prometheus metrics endpoint
- `/logging` - Log level management endpoint

## Configuration Options

### Server Ports

```go
srvr.WithExternalPort(8080)  // External API port
srvr.WithInternalPort(8081)  // Internal management port
```

### Routers

```go
srvr.WithExternalRouter(mux.NewRouter())  // Your API routes
srvr.WithInternalRouter(mux.NewRouter())  // Additional internal routes
```

### Timeouts

```go
srvr.WithReadTimeout(5 * time.Second)
srvr.WithWriteTimeout(10 * time.Second)
srvr.WithIdleTimeout(120 * time.Second)
srvr.WithReadHeaderTimeout(2 * time.Second)
srvr.WithMaxHeaderBytes(1 << 20)  // 1 MB
```

### Endpoint Customization

```go
srvr.WithLivenessManagement("/health/live")
srvr.WithReadinessManagement("/health/ready")
srvr.WithMetricsRoute("/prometheus")
srvr.WithLogManagement("/admin/logging")
```

### Logging Control

```go
srvr.WithInternalLogging(true)   // Log internal server requests
srvr.WithExternalLogging(false)  // Disable external server request logging
```

## Health Checks

### Adding Readiness Checks

Readiness checks determine if your application is ready to serve traffic:

```go
// Database readiness check
dbCheck := func(ctx context.Context) error {
    return db.PingContext(ctx)
}

// Redis readiness check
cacheCheck := func(ctx context.Context) error {
    return redisClient.Ping(ctx).Err()
}

server := srvr.Create(
    srvr.WithReadinessCheck("database", dbCheck),
    srvr.WithReadinessCheck("cache", cacheCheck),
    srvr.WithExternalRouter(router),
)
```

### Health Check Behavior

- **Liveness**: Always returns `200 OK` (indicates the process is running)
- **Readiness**: Returns `200 OK` only if all registered checks pass
- **Failed Readiness**: Returns `503 Service Unavailable` with error details

## Complete Example

```go
package main

import (
    "context"
    "database/sql"
    "net/http"
    "time"

    "github.com/gorilla/mux"
    "github.com/tpyle/srvr"
    _ "github.com/lib/pq"
)

func main() {
    // Setup database
    db, err := sql.Open("postgres", "postgresql://...")
    if err != nil {
        panic(err)
    }
    defer db.Close()

    // Create external router
    externalRouter := mux.NewRouter()
    externalRouter.HandleFunc("/api/users", handleUsers).Methods("GET")
    externalRouter.HandleFunc("/api/health", handleHealth).Methods("GET")

    // Create internal router (optional)
    internalRouter := mux.NewRouter()
    internalRouter.HandleFunc("/admin/status", handleAdminStatus).Methods("GET")

    // Database health check
    dbHealthCheck := func(ctx context.Context) error {
        return db.PingContext(ctx)
    }

    // Create server
    server := srvr.Create(
        // Ports
        srvr.WithExternalPort(8080),
        srvr.WithInternalPort(8081),

        // Routers
        srvr.WithExternalRouter(externalRouter),
        srvr.WithInternalRouter(internalRouter),

        // Timeouts
        srvr.WithReadTimeout(15 * time.Second),
        srvr.WithWriteTimeout(15 * time.Second),
        srvr.WithIdleTimeout(60 * time.Second),

        // Health checks
        srvr.WithReadinessCheck("database", dbHealthCheck),

        // Custom endpoint paths
        srvr.WithLivenessManagement("/health/live"),
        srvr.WithReadinessManagement("/health/ready"),
        srvr.WithMetricsRoute("/metrics"),

        // Logging
        srvr.WithExternalLogging(true),
        srvr.WithInternalLogging(false),
    )

    // Start server
    if err := server.Start(); err != nil {
        panic(err)
    }

    // Wait for interrupt signal
    // ... (signal handling code)

    // Graceful shutdown
    if err := server.Stop(); err != nil {
        log.Error().Err(err).Msg("Error stopping server")
    }
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte(`{"users": []}`))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("OK"))
}

func handleAdminStatus(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("Admin OK"))
}
```

## Observability Features

### Metrics

- Prometheus metrics are automatically collected for the external server
- Access metrics at `http://localhost:8081/metrics` (or your configured path)
- Includes standard HTTP metrics (request duration, response codes, etc.)

### Logging

- Structured logging using [zerolog](https://github.com/rs/zerolog)
- Request logging middleware for both servers (configurable)
- Dynamic log level management via `/logging` endpoint

### Health Checks

- Kubernetes-compatible liveness and readiness probes
- Custom readiness checks for dependencies
- Detailed error reporting for failed checks

## Server Lifecycle

```go
server := srvr.Create(options...)

// Start both servers (non-blocking)
err := server.Start()

// ... application runs ...

// Graceful shutdown
err = server.Stop()
```

## Dependencies

- [Gorilla Mux](https://github.com/gorilla/mux) - HTTP router
- [Prometheus Client](https://github.com/prometheus/client_golang) - Metrics
- [Zerolog](https://github.com/rs/zerolog) - Structured logging
- [Log Manager](https://github.com/tpyle/log-manager) - Dynamic log level management
- [Log Middleware](https://github.com/tpyle/log-middleware) - Request logging
