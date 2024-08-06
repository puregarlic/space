# Build styles
FROM node:22 AS build-styles
ENV PNPM_HOME="/pnpm"
ENV PATH="$PNPM_HOME:$PATH"
WORKDIR /app

COPY . /app

RUN corepack enable pnpm
RUN --mount=type=cache,id=pnpm,target=/pnpm/store pnpm install --frozen-lockfile
RUN pnpm run build

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

COPY etc/litestream.yml /etc/litestream.yml
COPY scripts/run.sh /scripts/run.sh

EXPOSE 80

COPY etc/litestream.yml /etc/litestream.yml
COPY scripts/run.sh /scripts/run.sh

CMD [ "/scripts/run.sh" ]
