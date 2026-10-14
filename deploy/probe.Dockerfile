FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/probe ./apps/probe-server
FROM alpine:3.23
RUN apk add --no-cache ca-certificates && addgroup -g 10001 flowlight && adduser -D -u 10001 -G flowlight flowlight
USER flowlight
COPY --from=build /out/probe /usr/local/bin/probe
EXPOSE 9080/tcp 9081/udp
ENTRYPOINT ["probe"]
CMD ["-http", "0.0.0.0:9080", "-udp", "0.0.0.0:9081"]
