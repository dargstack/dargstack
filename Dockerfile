FROM alpine:3.16.2@sha256:1304f174557314a7ed9eddb4eab12fed12cb0cd9809e4c28f29af86979a3c870

RUN apk update \
  && apk add --no-cache bash docker git sudo \
  && addgroup -g 1000 -S username \
  && adduser -u 1000 -S username -G username \
  && adduser username docker

COPY ./src/dargstack /bin/dargstack
COPY ./docker/entrypoint.sh /bin/docker-entrypoint.sh

ENV DOCKER=true

VOLUME ["/var/run/docker.sock"]
VOLUME ["/srv/app"]

USER 1000:1000

WORKDIR /srv/app

ENTRYPOINT ["docker-entrypoint.sh"]
