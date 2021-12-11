# sys-agent [![build](https://github.com/umputun/sys-agent/actions/workflows/ci.yml/badge.svg)](https://github.com/umputun/sys-agent/actions/workflows/ci.yml) [![Coverage Status](https://coveralls.io/repos/github/umputun/sys-agent/badge.svg?branch=master)](https://coveralls.io/github/umputun/sys-agent?branch=master)

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

   ```json
   {
     "hostname": "UMBP.localdomain",
     "procs": 697,
     "host_id": "021cd85f-69cc-5ae1-9725-5836eca1b092",
     "cpu_percent": 11,
     "mem_percent": 51,
     "uptime": 517719000000000,
     "volumes": {
       "root": {
         "name": "root",
         "path": "/",
         "usage_percent": 33
       },
        "data": {
           "name": "data",
           "path": "/mnt/data",
           "usage_percent": 87
        }
     },
    "load_average": {
      "one": 2.27294921875,
      "five": 3.52978515625,
      "fifteen": 3.76171875
    }
   }
   ```

 - `GET /ping` - returns `pong`
