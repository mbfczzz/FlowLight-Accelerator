FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/control ./apps/control-api
FROM alpine:3.23
RUN apk add --no-cache ca-certificates tzdata && addgroup -g 10001 flowlight && adduser -D -u 10001 -G flowlight flowlight && mkdir /data && chown flowlight:flowlight /data
USER flowlight
COPY --from=build /out/control /usr/local/bin/control
ENV DATA_DIR=/data LISTEN_ADDR=0.0.0.0:8090
EXPOSE 8090
ENTRYPOINT ["control"]
