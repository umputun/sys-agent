# sys-agent [![build](https://github.com/umputun/sys-agent/actions/workflows/ci.yml/badge.svg)](https://github.com/umputun/sys-agent/actions/workflows/ci.yml) [![Coverage Status](https://coveralls.io/repos/github/umputun/sys-agent/badge.svg?branch=master)](https://coveralls.io/github/umputun/sys-agent?branch=master)

System agent is a simple service reporting server status via HTTP GET request.

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
* services (`--service`, can be repeated) is a list of name:url pairs, where name is a name of the service, and url is a url to the service. Supports `http`, `https`, `mongodb` and `docker` schemes. The response for each service will be in `ext_services` field.
* concurrency (`--concurrency`) is a number of concurrent requests to services.
* timeout (`--timeout`) is a timeout for each request to services.

## api

 - `GET /status` - returns server status in JSON format
 - `GET /ping` - returns `pong`

### example

`$ sys-age -v root:/ -s "s1:https://echo.umputun.com/s1" -s "s2:https://echo.umputun.com/s2" --dbg`

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
  "ext_services": {
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
    }
  }
}
```
