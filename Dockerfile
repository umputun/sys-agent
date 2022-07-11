FROM umputun/baseimage:buildgo-latest  as build

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


FROM umputun/baseimage:scratch-latest
COPY --from=build /build/sys-agent /srv/sys-agent
WORKDIR /srv
EXPOSE 8080

CMD ["/srv/sys-agent"]