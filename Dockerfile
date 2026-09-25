# IoLink single-binary build. web/ is the iolink-webui submodule; its built
# dist/ is mirrored into internal/web/dist by scripts/embed-frontend.sh (which
# falls back to a placeholder page when the submodule has no dist yet) so that
# go:embed picks it up.
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN sh scripts/embed-frontend.sh
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/iolinkd ./cmd/iolinkd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/iolinkd /app/iolinkd
ENV IOLINK_HTTP_ADDR=:8080 IOLINK_MQTT_ADDR=:1883
EXPOSE 8080 1883
ENTRYPOINT ["/app/iolinkd"]
