# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

sys-agent is a simple status reporting server that monitors system metrics and external services via HTTP GET requests. It's designed for monitoring and debugging purposes, often used with monitoring systems like gatus.

The service reports:
- CPU, memory, disk utilization and load averages
- External service status via various providers (HTTP, MongoDB, Docker, Nginx, RMQ, etc.)
- Can run as a systemd service or Docker container

## Architecture

The codebase follows a clean modular structure:

- **app/main.go**: Entry point, handles CLI flags/env vars, sets up providers, starts REST server
- **app/config**: Configuration loading from YAML files
- **app/server**: HTTP REST server implementation using go-chi router with middleware
- **app/status**: Core status collection service that aggregates system metrics and external services
- **app/status/external**: External service providers implementing StatusProvider interface
  - Each provider (HTTP, Mongo, Docker, etc.) handles specific protocol checks
  - Service multiplexes concurrent requests to multiple providers
  - Support for cron-based scheduling and caching of responses
- **app/actuator**: Spring Boot Actuator compatible health response conversion

Key interfaces:
- `StatusProvider`: Interface for external service checkers
- `ExtServices`: Interface for aggregating all external service statuses
- `Status`: Interface for getting complete system status

## Build and Development Commands

```bash
# Run tests
go test ./...                               # run all tests
go test -v ./...                            # verbose output
go test -race ./...                         # test with race detection
go test -cover ./...                        # with coverage
cd app && go test -race -timeout=60s -count 1 ./...  # race test from Makefile

# Build
make build                                  # build linux amd64 binary to dist/
go build -o sys-agent ./app                # build for current platform

# Docker
make docker                                 # build docker image tagged as umputun/sys-agent:master
docker build -t umputun/sys-agent:latest . # build docker image

# Release
make release                                # create release binaries with goreleaser

# Linting (required before commits)
golangci-lint run                          # run linter from project root

# Development server
go run app/main.go -l :8080 -v "root:/" -s "health:http://localhost/health"
```

## Testing

Tests use MongoDB for integration testing. Set `MONGO_TEST` environment variable:
```bash
MONGO_TEST=mongodb://127.0.0.1:27017 go test ./...
```

The project uses testify for assertions and moq for mock generation:
- Mocks are generated with `//go:generate moq` directives
- Mock files follow `*_mock.go` naming convention

## Configuration

The service accepts configuration via:
1. Command-line flags (e.g., `-l :8080`, `-v root:/`)
2. Environment variables (e.g., `LISTEN=:8080`, `VOLUMES=root:/`)
3. YAML config file specified with `-f` or `CONFIG` env var

Config file structure mirrors CLI options with volumes and services definitions.

## API Endpoints

- `GET /status` - Returns complete system and service status in JSON
- `GET /actuator` - Actuator discovery endpoint with links to available endpoints
- `GET /actuator/health` - Spring Boot Actuator compatible health status
- `GET /actuator/health/{component}` - Health status of a specific component (e.g., cpu, memory, diskSpace:root)
- `GET /ping` - Health check endpoint (returns "pong")

The server includes middleware for:
- Rate limiting (10 req/s per IP via tollbooth)
- Request throttling (max 100 concurrent requests)
- Recovery from panics
- Request logging

## Provider URL Formats

Each external service provider uses specific URL format:
- HTTP/HTTPS: `name:http://example.com/health`
- MongoDB: `name:mongodb://user:pass@host:27017/?authSource=admin`
- Docker: `name:docker:///var/run/docker.sock?containers=nginx:redis`
- Program: `name:program:///path/to/script.sh`
- Nginx: `name:nginx://example.com:80/nginx_status`
- Certificate: `name:cert://example.com`
- File: `name:file:///path/to/file.txt`
- RMQ: `name:rmq://user:pass@host:15672/vhost/queue`

Providers support `cron` query parameter for scheduled checks.

## Dependencies

Key libraries used:
- `github.com/shirou/gopsutil/v3` - System metrics collection
- `github.com/go-pkgz/rest` & `github.com/go-pkgz/routegroup` - HTTP routing and middleware
- `github.com/go-pkgz/lgr` - Logging
- `github.com/umputun/go-flags` - CLI flag parsing
- `github.com/stretchr/testify` - Testing assertions
- `github.com/didip/tollbooth/v8` - Rate limiting
- `github.com/robfig/cron/v3` - Cron scheduling

## Important Implementation Details

- All providers implement timeout handling (default 5s)
- External services are checked concurrently (default concurrency: 4)
- Response caching for cron-scheduled checks
- MongoDB provider checks replica set status and oplog lag
- Docker provider validates container health status
- File provider tracks size and modification time changes
- Certificate provider warns about expiring certificates (5 days threshold)