FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN sh scripts/embed-frontend.sh
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /out/iolinkd ./cmd/iolinkd

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget
WORKDIR /app
COPY --from=build /out/iolinkd /app/iolinkd
ENV IOLINK_HTTP_ADDR=:8080 IOLINK_MQTT_ADDR=:1883
EXPOSE 8080 1883
ENTRYPOINT ["/app/iolinkd"]
