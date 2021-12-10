# sys-agent [![build](https://github.com/umputun/sys-agent/actions/workflows/ci.yml/badge.svg)](https://github.com/umputun/sys-agent/actions/workflows/ci.yml) [![Coverage Status](https://coveralls.io/repos/github/umputun/sys-agent/badge.svg?branch=main)](https://coveralls.io/github/umputun/sys-agent?branch=main) 

System agent is a simple service reporting server status via HTTP GET request.

## usage

`$ sys-agent -l :8080 -v "root:/" -v "data:/mnt/data"`


```
Application Options:
  -l, --listen= listen on host:port (default: localhost:8080) [$LISTEN]
  -v, --volume= volumes to report (default: root:/) [$VOLUMES]
      --dbg     show debug info [$DEBUG]

Help Options:
  -h, --help    Show this help message

```

## api

 - `GET /status` - returns server status

    response:
    ```json
    {
      "hostname": "UMBP.localdomain",
      "procs": 685,
      "host_id": "021af95f-69ca-5ae2-8725-5739eca1b094",
      "cpu_percent": 12,
      "volumes": [
        {
          "name": "root",
          "path": "/",
          "usage_percent": 33
        },
        {
          "name": "data",
          "path": "/mnt/data",
          "usage_percent": 67
        }
      ]
    }
    
    ```
 - `GET /ping` - returns `pong`
