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
RUN mkdir data
RUN mkdir data/docs

RUN CGO_ENABLED=0 GOOS=linux go build -o /space

FROM scratch
WORKDIR /
COPY --from=build-styles /app/static /static
COPY --from=build-server /app/data /data
COPY --from=build-server /space /space
EXPOSE 80
ENTRYPOINT ["/space"]
