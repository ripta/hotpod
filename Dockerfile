FROM golang:1.24 AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 go build -ldflags "-X main.version=${VERSION}" -o hotpod ./cmd/hotpod \
    && mkdir -p /tmp/hotpod

FROM gcr.io/distroless/static:nonroot

ARG VERSION=dev

LABEL org.opencontainers.image.title="hotpod"
LABEL org.opencontainers.image.description="Controllable load generation server for Kubernetes autoscaler testing"
LABEL org.opencontainers.image.source="https://github.com/ripta/hotpod"
LABEL org.opencontainers.image.version="${VERSION}"

COPY --from=builder /build/hotpod /hotpod
COPY --from=builder --chown=1001:1001 /tmp/hotpod /tmp/hotpod

USER 1001:1001

ENTRYPOINT ["/hotpod"]
