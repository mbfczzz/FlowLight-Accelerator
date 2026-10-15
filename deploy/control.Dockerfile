FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /out/control ./apps/control-api
# The node agent for each CPU a node may have; the control service hands it
# out with the install script (/dist/node/).
RUN for arch in amd64 arm64; do \
      CGO_ENABLED=0 GOOS=linux GOARCH=$arch go build -trimpath -o /out/node/$arch/flowlight-node ./agents/node-agent || exit 1; \
    done

# Pinned, checksum-verified ssserver, sslocal and realm for the same CPUs.
FROM alpine:3.23 AS node-tools
RUN apk add --no-cache curl xz
COPY deploy/node/fetch-tools.sh /fetch-tools.sh
RUN sh /fetch-tools.sh /out/node

FROM alpine:3.23
RUN apk add --no-cache ca-certificates tzdata && addgroup -g 10001 flowlight && adduser -D -u 10001 -G flowlight flowlight && mkdir /data && chown flowlight:flowlight /data
COPY --from=node-tools /out/node /opt/flowlight/node
COPY --from=build /out/node /opt/flowlight/node
USER flowlight
COPY --from=build /out/control /usr/local/bin/control
ENV DATA_DIR=/data LISTEN_ADDR=0.0.0.0:8090 NODE_DIST=/opt/flowlight/node
EXPOSE 8090
ENTRYPOINT ["control"]
