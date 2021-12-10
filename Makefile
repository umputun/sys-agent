B=$(shell git rev-parse --abbrev-ref HEAD)
BRANCH=$(subst /,-,$(B))
GITREV=$(shell git describe --abbrev=7 --always --tags)
REV=$(GITREV)-$(BRANCH)-$(shell date +%Y%m%d-%H:%M:%S)

docker:
	docker build -t umputun/sys-agent:master --progress=plain .

dist:
	- @mkdir -p dist
	docker build -f Dockerfile.artifacts --progress=plain -t sys-agent.bin .
	- @docker rm -f sys-agent.bin 2>/dev/null || exit 0
	docker run -d --name=sys-agent.bin sys-agent.bin
	docker cp sys-agent.bin:/artifacts dist/
	docker rm -f sys-agent.bin

race_test:
	cd app && go test -race -mod=vendor -timeout=60s -count 1 ./...

build: info
	- cd app && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.revision=$(REV) -s -w" -o ../dist/sys-agent

info:
	- @echo "revision $(REV)"

.PHONY: dist docker race_test bin info build_site
