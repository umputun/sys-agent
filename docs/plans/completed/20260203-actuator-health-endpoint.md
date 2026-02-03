# Actuator Health Endpoint

## Overview

Add Spring Boot Actuator-compatible `/actuator/health` endpoint to sys-agent for monitoring tool compatibility (gatus, uptime-kuma, etc.). This provides a standard health check format that monitoring tools expect.

**Issue**: #34

## Context

**Current `/status` response:**
```json
{
  "hostname": "host1",
  "cpu_percent": 25,
  "mem_percent": 50,
  "volumes": {"root": {"name": "root", "path": "/", "usage_percent": 45}},
  "services": {"mongo": {"name": "mongo", "status_code": 200, "response_time": 10}},
  "load_average": {"one": 1.5, "five": 1.2, "fifteen": 1.0}
}
```

**Target `/actuator/health` response:**
```json
{
  "status": "UP",
  "components": {
    "cpu": {"status": "UP", "details": {"percent": 25}},
    "memory": {"status": "UP", "details": {"percent": 50}},
    "diskSpace:root": {"status": "UP", "details": {"path": "/", "percent": 45}},
    "service:mongo": {"status": "UP", "details": {"status_code": 200, "response_time": 10}},
    "loadAverage": {"status": "UP", "details": {"one": 1.5, "five": 1.2, "fifteen": 1.0}}
  }
}
```

**Files involved:**
- `app/server/server.go` - add new route
- `app/status/status.go` - existing Info struct (data source)
- New: `app/actuator/actuator.go` - conversion logic

## Development Approach

- **Testing approach**: TDD (tests first)
- Complete each task fully before moving to the next
- Every task includes tests as required deliverable
- All tests must pass before starting next task

## Implementation Steps

### Task 1: Create actuator types and tests

- [x] create `app/actuator/actuator.go` with types: `HealthResponse`, `Component`
- [x] create `app/actuator/actuator_test.go` with test cases for conversion logic
- [x] run tests - they should fail (no implementation yet)

### Task 2: Implement conversion logic (TDD green)

- [x] implement `FromStatusInfo(info *status.Info) *HealthResponse` function
- [x] implement status determination logic (UP if cpu/mem/disk < 90%, service 2xx)
- [x] run tests - must pass before next task

### Task 3: Add /actuator/health endpoint

- [x] write test in `app/server/server_test.go` for `GET /actuator/health`
- [x] add route in `app/server/server.go` calling actuator conversion
- [x] run tests - must pass before next task

### Task 4: Verify and document

- [x] run full test suite: `go test ./...`
- [x] run linter: `golangci-lint run`
- [x] manual test: `curl http://localhost:8080/actuator/health | jq`
- [x] update README.md with new endpoint documentation
- [x] move plan to `docs/plans/completed/`

## Technical Details

**Status determination:**
- `UP`: cpu < 90%, mem < 90%, disk < 90%, service status_code 200-299
- `DOWN`: any threshold exceeded or service error

**Component mapping:**
| sys-agent field | Actuator component | Details |
|-----------------|-------------------|---------|
| `cpu_percent` | `cpu` | `percent` |
| `mem_percent` | `memory` | `percent` |
| `volumes[name]` | `diskSpace:{name}` | `path`, `percent` |
| `services[name]` | `service:{name}` | `status_code`, `response_time`, `body` |
| `load_average` | `loadAverage` | `one`, `five`, `fifteen` |

## Post-Completion

**Manual verification:**
- Test with gatus or similar monitoring tool
- Verify response matches Spring Boot actuator format
