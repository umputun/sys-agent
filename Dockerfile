FROM umputun/baseimage:buildgo-v1.7.0  as build

ARG GIT_BRANCH
ARG GITHUB_SHA
ARG CI

ENV GOFLAGS="-mod=vendor"

ADD . /build
WORKDIR /build

RUN \
    if [ -z "$CI" ] ; then \
    echo "runs outside of CI" && version=$(git rev-parse --abbrev-ref HEAD)-$(git log -1 --format=%h)-$(date +%Y%m%dT%H:%M:%S); \
    else version=${GIT_BRANCH}-${GITHUB_SHA:0:7}-$(date +%Y%m%dT%H:%M:%S); fi && \
    echo "version=$version" && \
    cd app && go build -o /build/sys-agent -ldflags "-X main.revision=${version} -s -w"


FROM ghcr.io/umputun/baseimage/app:v1.7.0
COPY --from=build /build/sys-agent /srv/sys-agent
WORKDIR /srv
CMD ["/srv/sys-agent"]
