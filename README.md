# sys-agent [![build](https://github.com/umputun/sys-agent/actions/workflows/ci.yml/badge.svg)](https://github.com/umputun/sys-agent/actions/workflows/ci.yml) [![Coverage Status](https://coveralls.io/repos/github/umputun/sys-agent/badge.svg?branch=master)](https://coveralls.io/github/umputun/sys-agent?branch=master)

<div align="center">
  <img class="logo" src="https://raw.githubusercontent.com/umputun/sys-agent/master/site/docs/logo.png" width="500px" height="132px" alt="SysAgent | Simple Status Reporting Server"/>
</div>

SysAgent is a simple service reporting server status via HTTP GET request. It is useful for monitoring and debugging purposes, but usually used as a part of some other monitoring system collecting data and serving it. One of such systems is [gatus](https://github.com/TwiN/gatus), and it works fine with `sys-agent`.

`sys-agent` can run directly on a server (systemd service provided) or as a docker container (multi-arch container provided).

All the configuration is done via a few command line options/environment variables. Generally, the user should define a list of data volumes to be reported and optional external services to be checked. Volumes report capacity/utilization. CPU-related metrics, like LAs, overall utilization, and the number of running processes are always reported, as well as memory usage.

The idea of external services is to be able to integrate the status of all related services into a single response. This way a single JSON response can report instance metrics as well as the status of HTTP health check, the status of running containers, etc.

## installation

- install binary from [releases](https://github.com/umputun/sys-agent/releases/). It has amd64, arm64 and armv7 builds for deb, rpm and apk packages as well as for tar.gz archive.
- it also has brew package for macos: `brew install sys-agent`.
- for docker use `umputun/sys-agent:latest` or `ghcr.io/umputun/sys-agent:latest` image. It is a multi-arch image with amd64 and arm64 builds.

## usage

`$ sys-agent -l :8080 -v "root:/" -v "data:/mnt/data"`


```
Application Options:
  -f, --config=      config file [$CONFIG]
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
* config file (`--config`, `-f`) is a path to the config file, see below for details.

## configuration file 

`sys-agent` can be configured with a yaml file as well. The file should contain a list of volumes and services. The file can be specified via `--config` or `-f` options or `CONFIG` environment variable. 

```yml
volumes:
  - {name: root, path: /hostroot}
  - {name: data, path: /data}

services:
  mongo:
    - {name: dev, url: mongodb://example.com:27017, oplog_max_delta: 30m}
    # Optional: check document count with db, collection and count_query
    # - {name: test, url: mongodb://example.com:27017, db: testdb, collection: users, count_query: '{"status":"active"}'}
  certificate:
    - {name: prim_cert, url: https://example1.com}
    - {name: second_cert, url: https://example2.com}
  docker:
    - {name: docker1, url: unix:///var/run/docker.sock, containers: [reproxy, mattermost, postgres]}
    - {name: docker2, url: tcp://192.168.1.1:4080}
  file:
    - {name: first, path: /tmp/example1.txt}
    - {name: second, path: /tmp/example2.txt}
  http:
    - {name: first, url: https://example1.com}
    - {name: second, url: https://example2.com}
  program:
    - {name: first, path: /usr/bin/example1, args: [arg1, arg2]}
    - {name: second, path: /usr/bin/example2}
  nginx:
    - {name: nginx, status_url: http://example.com:80}
  rmq:
    - {name: rmqtest, url: http://example.com:15672, vhost: v1, queue: q1, user: guest, pass: passwd}
```

The config file has the same structure as command line options. `sys-agent` converts the config file to command line options and then parses them as usual. 

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

In addition to the basic checks `sys-agent` can report the status of external services. Each service is defined as a "name:url" pair for supported protocols (`http`, `mongodb`, `docker`, `file`, `nginx`, `cert`, `rmq` and `program`). Each service will be reported as a separate element in the response, and all responses have a similar structure: `name` (service name), `status_code` (`200` or `4xx`), and `response_time` in milliseconds. The `body` includes the response details JSON, different for each service.

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

Check if MongoDB is available and report the status of the replica set (for non-standalone configurations only). All the nodes should be in a valid state, and the oplog time difference should be less than 60 seconds by default. Users can change the default via the `oplogMaxDelta` query parameter.

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

In addition, `mongodb` can also check count of documents in a collection for a given query. In this case it adds `count` field to the response body.

Request example: `foo:mongodb://example.com:27017/admin?db=test&collection=blah&count={\"status\":\"active\"}`

In some cases, requests should be limited by a specific date range. In this situation, the query can include `[[.YYYYMMDD]]` and `[[.YYYYMMDD1]]` to `[[.YYYYMMDD5]]` template placeholders. These will be replaced with the current date and the dates of the previous days, such as 1 day ago, 2 days ago, and so on. It is also useful to check the count of documents for the last N minutes or hours from now by using the following placeholders:

-  `[[.NOW]]`    - current time, with seconds precision
-  `[[.NOW1M]]`  - current time -1 minute, with seconds precision
-  `[[.NOW5M]]`  - current time -5 minutes, with seconds precision
-  `[[.NOW10M]]` - current time -10 minutes, with seconds precision
-  `[[.NOW15M]]` - current time -15 minutes, with seconds precision
-  `[[.NOW30M]]` - current time -30 minutes, with seconds precision
-  `[[.NOW1H]]`  - current time -1 hour, with seconds precision
-  `[[.NOW5H]]`  - current time -5 hours, with seconds precision
-  `[[.NOW12H]]` - current time -12 hours, with seconds precision

request example: `foo:mongodb://example.com:27017/admin?db=test&collection=blah&count={\"status\":\"active\",\"created_at\":{\"$gte\":\"[[.NOW1H]]\"}}`
#### `docker` provider

Check if the Docker service is available and if the required container (optional) is running. The `containers` parameter is a list of required container names separated by `:`.

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

This check runs a request to the nginx status page, checks, and parses the response. In order to use this provider, you need to have nginx with the `stub_status` enabled.

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

Checks if the certificate has expired or is going to expire in the next 5 days.

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
      "days_left": 73,
      "expire": "2022-09-03T16:31:52Z",
      "status": "ok"
    }
  }
}
```

#### `file` provider

Check if the file is present and set stats info

Request examples:
- `foo:file://foo/bar.txt` - Check if a file with a relative path exists and set stats info
- `bar:file:///srv/foo/bar.txt` - Check if a file with the absolute path exists and set stats info


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
      "since_modif": 678900,
      "size_change": 1234,
      "modif_change": 200
    }
  }
}
```

In addition to the current file status, this provider also keeps track of the difference between the current and previous file size and modification time and sets the following values: `size_change` (in bytes) and `modif_change` (in milliseconds).

#### `rmq` provider

Gets stats from RabbitMQ management API.

Request examples:
- `foo:rmq://user:passwd@example.com:1234/foo/vhost1/queue1` - returns stats for queue1 in vhost1


- Response example:

```json
{
  "rmq": {
    "name": "rmq-test",
    "status_code": 200,
    "response_time": 12,
    "body": {
      "avg_egress_rate":15.5,
      "avg_ingress_rate":19.9,
      "consumers":4,
      "messages":56178,
      "messages_delta":578,
      "messages_rate":11.06,
      "messages_ready":56178,
      "messages_ready_ram":3771,
      "messages_unacknowledged":0,
      "name": "notification.queue",
      "publish":13847734,
      "publish_rate":0,
      "state":"running",
      "vhost":"feeds"
    }
  }
}
```

In addition to the current status, this provider also keeps track of the difference between current and previous number of messages in `messages_delta`.

### using `cron` parameter to limit provider checks

Each provider url can contain `cron` query parameter to limit checks to specific time. The parameter is a cron expression in the following format: `cron=0 0 * * * *` (seconds, minutes, hours, day of month, month, day of week). Instead of spaces either `+` or `_` can be used.

If the given provider has `cron` parameter and the current time does not match the cron expression, the provider will be skipped. In this case, the response will be returned from the local cache with the last check response.

example: `https://example.com/s1?cron=0_7-18_*_*_*`

## API

 - `GET /status` - returns server status in JSON format
 - `GET /actuator/health` - returns Spring Boot Actuator compatible health status
 - `GET /ping` - returns `pong`

### /actuator/health endpoint

The `/actuator/health` endpoint provides Spring Boot Actuator compatible health status, making it easy to integrate with monitoring tools that expect the actuator format (gatus, uptime-kuma, etc.).

**Status determination:**
- CPU, memory, disk: `UP` if usage < 90%, `DOWN` otherwise
- External services: `UP` if status code is 2xx, `DOWN` otherwise
- Overall status: `DOWN` if any component is `DOWN`, `UP` otherwise

**Response example:**

```json
{
  "status": "UP",
  "components": {
    "cpu": {
      "status": "UP",
      "details": {"percent": 25}
    },
    "memory": {
      "status": "UP",
      "details": {"percent": 50}
    },
    "diskSpace:root": {
      "status": "UP",
      "details": {"path": "/", "percent": 45}
    },
    "service:mongo": {
      "status": "UP",
      "details": {"status_code": 200, "response_time": 10}
    },
    "loadAverage": {
      "status": "UP",
      "details": {"one": 1.5, "five": 1.2, "fifteen": 1.0}
    }
  }
}
```

### /status example

```
$ sys-agent -v root:/ -s "s1:https://echo.umputun.com/s1" -s "s2:https://echo.umputun.com/s2?cron=*_9-18_*_*_*" \
 -s mongo:mongodb://1.2.3.4:27017/ -s docker:docker:///var/run/docker.sock --dbg
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
