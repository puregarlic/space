# Build styles
FROM denoland/deno:1.45.5 AS build-styles
WORKDIR /app

COPY . /app

RUN deno run --allow-all npm:tailwindcss -o static/styles.css -c config/tailwind.config.js --minify

FROM golang:1.22-alpine AS build-server
WORKDIR /app

COPY . /app

RUN go mod download

ENV CGO_ENABLED=0
ENV GOOS=linux
RUN go build -ldflags '-s -w -extldflags "-static"' -tags osusergo,netgo,sqlite_omit_load_extension -o /space

ADD https://github.com/benbjohnson/litestream/releases/download/v0.3.13/litestream-v0.3.13-linux-amd64.tar.gz /tmp/litestream.tar.gz
RUN tar -C /usr/local/bin -xzf /tmp/litestream.tar.gz

FROM alpine
WORKDIR /

COPY --from=build-styles /app/static /static
COPY --from=build-server /space /space
COPY --from=build-server /usr/local/bin/litestream /usr/local/bin/litestream

RUN apk add bash
RUN mkdir -p /data

EXPOSE 80

COPY config/litestream.yml /etc/litestream.yml
COPY scripts/run.sh /scripts/run.sh

CMD [ "/scripts/run.sh" ]
