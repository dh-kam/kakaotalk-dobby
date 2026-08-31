# syntax=docker/dockerfile:1

ARG GO_VERSION=1.25
ARG ALPINE_VERSION=3.22

FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
ARG TARGETOS
ARG TARGETARCH
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath \
        -ldflags="-s -w \
            -X github.com/dh-kam/kakao-bot/internal/buildinfo.Version=${VERSION} \
            -X github.com/dh-kam/kakao-bot/internal/buildinfo.Commit=${COMMIT} \
            -X github.com/dh-kam/kakao-bot/internal/buildinfo.Date=${DATE}" \
        -o /out/kakaobot .

FROM alpine:${ALPINE_VERSION}

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S kakaobot \
    && adduser -S -G kakaobot -u 10001 kakaobot \
    && mkdir -p /home/kakaobot/.config/kakao-bot \
    && chown -R kakaobot:kakaobot /home/kakaobot

COPY --from=builder /out/kakaobot /usr/local/bin/kakaobot

USER kakaobot

# OAuth tokens are persisted at ~/.config/kakao-bot/tokens.json by default;
# mount a volume over /home/kakaobot/.config/kakao-bot to keep them.
WORKDIR /home/kakaobot

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/usr/local/bin/kakaobot"]
CMD ["serve", "--listen", "0.0.0.0:8080"]
