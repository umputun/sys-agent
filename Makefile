TAG=$(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null)
BRANCH=$(if $(TAG),$(TAG),$(shell git rev-parse --abbrev-ref HEAD 2>/dev/null))
HASH=$(shell git rev-parse --short=7 HEAD 2>/dev/null)
TIMESTAMP=$(shell git log -1 --format=%ct HEAD 2>/dev/null | xargs -I{} date -u -r {} +%Y%m%dT%H%M%S)
GIT_REV=$(shell printf "%s-%s-%s" "$(BRANCH)" "$(HASH)" "$(TIMESTAMP)")
REV=$(if $(filter --,$(GIT_REV)),latest,$(GIT_REV)) # fallback to latest if not in git repo

docker:
	docker build -t umputun/sys-agent:master --progress=plain .

release:
	@echo release to .bin
	goreleaser --snapshot --skip-publish --clean
	ls -l .bin

site:
	@rm -f  site/public/*
	@docker rm -f sys-agent-site
	docker build -f Dockerfile.site --progress=plain -t sys-agent.site .
	docker run -d --name=sys-agent-site sys-agent.site
	sleep 3
	docker cp "sys-agent-site":/srv/site/ site/public
	docker rm -f sys-agent-site

race_test:
	cd app && go test -race -timeout=60s -count 1 ./...

build: info
	- cd app && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.revision=$(REV) -s -w" -o ../dist/sys-agent

info:
	- @echo "revision $(REV)"

.PHONY: dist docker race_test bin info site
