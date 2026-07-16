FROM golang:1.26.4-bookworm AS build
WORKDIR /src
ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG GOPROXY=https://proxy.golang.org,direct
ARG BUILD_VERSION=go-dev
ARG BUILD_COMMIT=local
ARG BUILD_TIME=
ENV GOPROXY=$GOPROXY
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath \
    -ldflags "-s -w -X aipi-go/internal/build.Version=$BUILD_VERSION -X aipi-go/internal/build.Commit=$BUILD_COMMIT -X aipi-go/internal/build.Time=$BUILD_TIME" \
    -o /out/ai-pai ./cmd/ai-pai

FROM debian:bookworm-slim AS runtime
WORKDIR /app
ENV PORT=3001 SERVE_STATIC=false LOG_DIR=/app/logs TZ=Asia/Shanghai
RUN mkdir -p /app/logs
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=build /out/ai-pai /app/ai-pai
EXPOSE 3001
CMD ["/app/ai-pai"]
