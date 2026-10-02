ARG GO_BUILD_IMAGE=golang:1.26-alpine
ARG GO_RUNTIME_IMAGE=alpine:3.22

FROM ${GO_BUILD_IMAGE} AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/logistics-api ./cmd/server \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/logistics-worker ./cmd/worker \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/logistics-migrator ./cmd/migrate

FROM ${GO_RUNTIME_IMAGE} AS runtime-base

RUN apk add --no-cache ca-certificates \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app
USER app

FROM runtime-base AS api
COPY --from=build /out/logistics-api /usr/local/bin/logistics-api
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/logistics-api"]

FROM runtime-base AS worker
COPY --from=build /out/logistics-worker /usr/local/bin/logistics-worker
ENTRYPOINT ["/usr/local/bin/logistics-worker"]

FROM runtime-base AS migrator
COPY --from=build /out/logistics-migrator /usr/local/bin/logistics-migrator
ENTRYPOINT ["/usr/local/bin/logistics-migrator"]
