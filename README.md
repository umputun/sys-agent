# sys-agent [![build](https://github.com/umputun/sys-agent/actions/workflows/ci.yml/badge.svg)](https://github.com/umputun/sys-agent/actions/workflows/ci.yml) [![Coverage Status](https://coveralls.io/repos/github/umputun/sys-agent/badge.svg?branch=master)](https://coveralls.io/github/umputun/sys-agent?branch=master)

System agent is a simple service reporting server status via HTTP GET request. It is useful for monitoring and debugging purposes, but usually used as a part of some other monitoring system collecting data and serving it. One of such systems is [gatus](https://github.com/TwiN/gatus), and it works fine with `sys-agent`.

`sys-agent` can run directly on a server (systemd service provided) or as a docker container (multi-arch container provided).

All the configuration is done via a few command line options/environment variables. Generally, user should define a list of data volumes to be reported and optional external services to be checked. Volumes report capacity/utilization. CPU related metrics, like LAs, overall utilization and number of running processes are always reported, as well as memory usage.

The idea of external services is to be able to integrate status of all related services into a single response. This way a singe json response can report instance metrics as well as status of http health check, status of running containers, etc.

## usage

`$ sys-agent -l :8080 -v "root:/" -v "data:/mnt/data"`


```
Application Options:
  -l, --listen= listen on host:port (default: localhost:8080) [$LISTEN]
  -v, --volume= volumes to report (default: root:/) [$VOLUMES]
  -s, --service= services to report [$SERVICES]  
      --concurrency= number of concurrent requests to services (default: 4) [$CONCURRENCY]
      --timeout= timeout for each request to services (default: 5s) [$TIMEOUT] 
      --dbg     show debug info [$DEBUG]

Help Options:
  -h, --help    Show this help message

```
### parameters details

* volumes (`--volume`, can be repeated) is a list of name:path pairs, where name is a name of the volume, and path is a path to the volume.
* services (`--service`, can be repeated) is a list of name:url pairs, where name is a name of the service, and url is a url to the service. Supports `http`, `https`, `mongodb` and `docker` schemes. The response for each service will be in `services` field.
* concurrency (`--concurrency`) is a number of concurrent requests to services.
* timeout (`--timeout`) is a timeout for each request to services.

## basic checks

`sys-agent` always reports  internal metrics for cpu, memory, volumes and load averages.

```json
{
  "hostname": "BigMac.localdomain",
  "procs": 723,
  "host_id": "cd9973a05-85e7-5bca0-b393-5285825e3556",
  "cpu_percent": 7,
  "mem_percent": 49,
  "uptime": 99780,
  "volumes": {
    "root": {
      "name": "root",
      "path": "/",
      "usage_percent": 78
    }
  },
  "load_average": {
    "one": 3.52978515625,
    "five": 3.43359375,
    "fifteen": 3.33203125
  }
}
```

## external services

In addition to the basic checks `sys-agent` can report status of external services. Each service defined as name:url pair for supported protocols (`http`,`, `mongodb` and `docker`). Each servce will be reported as a separate element in the response and all responses have the similar structure: `name` (service name),  `status_code` (`200` or `4xx`) and `response_time` in milliseconds. The `body` includes the response details json, different for each service.

### service providers (protocols)

#### `http` and `https` provider

Checks if service is available by GET request. 

Request example: `health:https://example.com/ping`

Response example:

```json
{
  "web": {
    "body": {
      "text": "pong"
    },
    "name": "web",
    "response_time": 109,
    "status_code": 200
  }
}
```

note: `body.text` field will include the original response body if response is not json. If response is json the `body` will contain the parsed json. 

#### `mongodb` provider

Checks if mongo available and report status of replica set (for non-standalone configurations only). All the nodes should be in valid state and oplog time difference should be less than 60 seconds by default. User can change the default via `oplogMaxDelta` query parameter.

Request examples:
- `foo:mongodb://example.com:27017/` - check if mongo is available, no authentication
- `bar:mongodb://user:password@example.com:27017/?authSource=admin` - check if mongo is available with authentication
- `baz:mongodb://example.com:27017/?oplogMaxDelta=30s` - check if mongo is available and oplog difference between primary and secondary is less than 30 seconds

_see [mongo connection-string](https://docs.mongodb.com/manual/reference/connection-string/) for more details_

Response example:

```json
{
  "mongo": {
    "name": "foo",
    "status_code": 200,
    "response_time": 44,
    "body": {
      "rs": {
        "status": "ok",
        "optime:": "ok",
        "info": {
          "set":"rs1",
          "ok":1,
          "members":[
            {"name":"node1.example.com:27017","state":"PRIMARY","optime":{"ts":"2022-02-03T08:47:37Z"}},
            {"name":"node2.example.com:27017","state":"SECONDARY","optime":{"ts":"2022-02-03T08:47:37Z"}},
            {"name":"node3.example.com:27017","state":"ARBITER","optime":{"ts":"0001-01-01T00:00:00Z"}}]},
        }
      }
    }
}
```

- `rs.status` ("ok" or "failed") indicates if replica set is available and in valid state
- `rs.optime` ("ok" or "failed") indicates if oplog time difference is less than 60 seconds or defined `oplogMaxDelta`

The rest of details is a subset of the [replica status](https://docs.mongodb.com/manual/reference/command/replSetGetStatus/)

#### `docker` provider

Checks if docker service is available and required container (optional) are running.  The `containers` parameter is a list of required container names separated by `:`

Request examples:
- `foo:docker://example.com:2375/` - check if docker is available
- `bar:docker:///var/run/docker.sock?containers=nginx:redis` - check if docker is available and `nginx` and `redis` containers are running

- Response example:

```json
{
  "docker": {
    "body": {
      "containers": {
        "consul": {
          "name": "consul",
          "state": "running",
          "status": "Up 3 months (healthy)"
        },
        "logger": {
          "name": "logger",
          "state": "running",
          "status": "Up 3 months"
        },
        "nginx": {
          "name": "nginx",
          "state": "running",
          "status": "Up 3 months"
        },
        "registry-v2": {
          "name": "registry-v2",
          "state": "running",
          "status": "Up 3 months"
        }
      },
      "failed": 0,
      "healthy": 1,
      "required": "ok",
      "running": 4,
      "total": 4,
      "unhealthy": 0
    },
    "name": "docker",
    "response_time": 2,
    "status_code": 200
  }
}
```

- `docker.body.failed` - number of failed or non-running containers
- `docker.body.healthy` - number of healthy containers, only for those with health check
- `docker.body.unhealthy` - number of unhealthy containers, only for those with health check
- `docker.body.required` - "ok" if all required containers are running, otherwise "failed" with a list of failed containers

#### `program` provider

This check runs any predefined program/script and checks the exit code. All commands are executed in shell.

Request examples:
- `foo:program://ps?args=-ef` - runs `ps -ef` and checks exit code
- `bar:program:///tmp/foo/bar.sh` - runs /tmp/foo/bar.sh and checks exit code

- Response example:

```json
{
  "program": {
    "name": "foo",
    "status_code": 200,
    "response_time": 44,
    "body": {
      "command": "ps -ef",
      "stdout": "some output",
      "status": "ok"
    }
  }
}
```

#### `nginx` provider

This check runs request to nginx status page, checks and parse the response. In order to use this provider you need to have nginx with enabled `stub_status`.

```nginx
    location /nginx_status {
        stub_status on;
        access_log   off;
    }
```
request examples: `nginx-status:nginx://example.com:8080/nginx_status` 

This provider parses the nginx's response and returns the following:

```json
{
  "nginx": {
    "name": "nginx-status",
    "status_code": 200,
    "response_time": 12,
    "body": {
      "active_connections": 123,
      "accepts": 456,
      "handled": 789,
      "requests": 101112,
      "reading": 131,
      "writing": 132,
      "change_handled": 111,
    }
  }
}
```

All the values are parsed directly from the response except `change_handled` which is a difference between two subsequent `handled` values.

#### `certificate` provider

Checks if certificate expired or going to expire in the next 5 days.

Request examples:
- `foo:cert://example.com` - check if certificate is ok for https://example.com
- `bar:cert://umputun.com` - check if certificate is ok for https://umputun.com


- Response example:

```json
{
  "cert": {
    "name": "bar",
    "status_code": 200,
    "response_time": 44,
    "body": {
      "days_left": 73,,
      "expire": "2022-09-03T16:31:52Z",
      "status": "ok"
    }
  }
}
```

#### `file` provider

Checks if file present and sets stats info

Request examples:
- `foo:file://foo/bar.txt` - check if file with relative path exists and sets stats info
- `bar:cert:///srv/foo/bar.txt` - check if file with absolute path exists and sets stats info


- Response example:

```json
{
  "cert": {
    "name": "bar",
    "status_code": 200,
    "response_time": 44,
    "body": {
      "status": "found",
      "modif_time": "2022-07-11T16:12:03.674378878-05:00",
      "size": 1234,
      "since_modif": 678900
    }
  }
}
```

## API

 - `GET /status` - returns server status in JSON format
 - `GET /ping` - returns `pong`

### example

```
$ sys-age -v root:/ -s "s1:https://echo.umputun.com/s1" -s "s2:https://echo.umputun.com/s2" \
 -s mongo://mongodb://1.2.3.4:27017/ -s docker:///var/run/docker.sock --dbg`
```

request: `curl -s http://localhost:8080/status`

response:

```json
{
  "hostname": "BigMac.localdomain",
  "procs": 723,
  "host_id": "cd9973a05-85e7-5bca0-b393-5285825e3556",
  "cpu_percent": 7,
  "mem_percent": 49,
  "uptime": 99780,
  "volumes": {
    "root": {
      "name": "root",
      "path": "/",
      "usage_percent": 78
    }
  }, 
  "load_average": {
      "one": 3.52978515625,
      "five": 3.43359375,
      "fifteen": 3.33203125
 },
  "services": {
    "s1": {
      "name": "s1",
      "status_code": 200,
      "response_time": 595,
      "body": {
        "headers": {
          "Accept-Encoding": "gzip",
          "User-Agent": "Go-http-client/2.0",
          "X-Forwarded-For": "67.201.40.233",
          "X-Forwarded-Host": "echo.umputun.com",
          "X-Real-Ip": "67.201.40.233"
        },
        "host": "172.28.0.2:8080",
        "message": "echo echo 123",
        "remote_addr": "172.28.0.7:49690",
        "request": "GET /s1"
      }
    },
    "s2": {
      "name": "s2",
      "status_code": 200,
      "response_time": 595,
      "body": {
        "headers": {
          "Accept-Encoding": "gzip",
          "User-Agent": "Go-http-client/2.0",
          "X-Forwarded-For": "67.201.40.233",
          "X-Forwarded-Host": "echo.umputun.com",
          "X-Real-Ip": "67.201.40.233"
        },
        "host": "172.28.0.2:8080",
        "message": "echo echo 123",
        "remote_addr": "172.28.0.7:49692",
        "request": "GET /s2"
      }
    },
    "docker": {
      "body": {
        "containers": {
          "consul": {
            "name": "consul",
            "state": "running",
            "status": "Up 7 weeks (healthy)"
          },
          "logger": {
            "name": "logger",
            "state": "running",
            "status": "Up 7 weeks"
          },
          "nginx": {
            "name": "nginx",
            "state": "running",
            "status": "Up 13 days"
          },
          "sys-agent": {
            "name": "sys-agent",
            "state": "running",
            "status": "Up 7 hours"
          }
        },
        "failed": 0,
        "healthy": 1,
        "running": 4,
        "total": 4
      },
      "name": "docker",
      "response_time": 5,
      "status_code": 200,
      "required": "ok"
    },
    "mongo": {
      "name": "mongo",
      "status_code": 200,
      "response_time": 4,
      "body": {"status":"ok"}
    }
  }
}
```

## running sys-agent in docker

`sys-agent` is capable of running directly on a box as well as from docker container. For the direct run both binary archives and install packages are available. For docker run you need to map volumes, and it is recommended to mount them in `ro` mode. Example of a docker compose file:

```
services:
  sys-agent:
    image: umputun/sys-agent:latest
    container_name: sys-agent
    hostname: sys-agent
    ports:
      - "8080:8080"
    volumes:
      - /home:/hosthome:ro
      - /:/hostroot:ro
      - /var/run/docker.sock:/var/run/docker.sock:ro
    environment:
      - LISTEN=0.0.0.0:8080
      - VOLUMES=home:/hosthome,root:/hostroot
      - SERVICES=health:http://172.17.42.1/health,docker:docker:///var/run/docker.sock

```

## example of using `sys-agent` with [gatus](https://github.com/TwiN/gatus)

this is a gatus configuration example:

```yml
  - name: web-site
    group: things
    url: "http://10.0.0.244:4041/status"
    interval: 1m
    conditions:
      - "[STATUS] == 200"
      - "[BODY].volumes.root.usage_percent < 95"
      - "[BODY].volumes.data.usage_percent < 95"
      - "[BODY].services.docker.body.failed == 0"
      - "[BODY].services.docker.body.running > 3"
      - "[BODY].services.docker.body.required  == ok"
      - "[BODY].services.web.status_code == 200"
      - "[BODY].services.web.response_time < 100"
    alerts:
      - type: slack
  ```

`sys-agent` command line used for this example: 

```
sys-agent -l :4041 -v root:/ -v data:/data -s docker:docker:///var/run/docker.sock -s web:https://echo.umputun.com/foo/bar
```

## credits

- `sys-agent` is using a very nice and functional [github.com/shirou/gopsutil/v3](https://github.com/shirou/gopsutil) (psutil for golang) package to collect cpu, memory and volume statuses.
- http api served with indispensable [chi](https://github.com/go-chi/chi) web router.
