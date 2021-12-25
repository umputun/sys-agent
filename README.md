# sys-agent [![build](https://github.com/umputun/sys-agent/actions/workflows/ci.yml/badge.svg)](https://github.com/umputun/sys-agent/actions/workflows/ci.yml) [![Coverage Status](https://coveralls.io/repos/github/umputun/sys-agent/badge.svg?branch=master)](https://coveralls.io/github/umputun/sys-agent?branch=master)

System agent is a simple service reporting server status via HTTP GET request. It is useful for monitoring and debugging purposes, but usually as a part of some other monitoring system collecting data and serving it. One of such systems is [gatus,](https://github.com/TwiN/gatus) and it works fine with sys-agent.

`sys-agent` can run directly on a server (systemd service provided) or as a docker container (mutli-arch container provided).

All the configuration is done via a few command line options/environment variables. Generally, user should define a list of data volumes to be reported and optional external services to be checked. Volumes report capacity/utilization. CPU related metrics, like LAs, overall utilization and number of running processes are always reported, as well as memory usage.

The idea of external services is to be able to integrate status of all related services into a single response. This way a singe json response can report instance metrice as well as status of http health check, status of running containers, etc.

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

## external services

* `http` and `https` - checks if service is available by GET request, i.e. `health:http://example.com/ping`. Response example:
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
* `mongodb` - checks if mongo available, i.e. `foo:mongodb://example.com:27017/`
* `docker` - checks if docker service is available, i.e. `bar:docker:///var/run/docker.sock?containers=nginx:redis`. The `containers` parameter is a list of required container names separated by `:`, optional. Response example:
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
  - failed - number of failed or non-running containers
  - healthy - number of healthy containers, only for those with health check
  - unhealthy - number of unhealthy containers, only for those with health check
  - required - ok if all required containers are running, otherwise failed with a list of failed containers

## api

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
