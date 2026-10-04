FROM node:24-alpine AS frontend
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
COPY --from=frontend /web/dist ./web/dist
RUN sh scripts/embed-frontend.sh
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/iolinkd ./cmd/iolinkd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget
WORKDIR /app
COPY --from=build /out/iolinkd /app/iolinkd
ENV IOLINK_HTTP_ADDR=:8080 IOLINK_MQTT_ADDR=:1883
EXPOSE 8080 1883
ENTRYPOINT ["/app/iolinkd"]
