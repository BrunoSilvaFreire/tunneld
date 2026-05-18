FROM golang:1.26-alpine AS build

WORKDIR /src
ENV GONOSUMDB=github.com/BrunoSilvaFreire

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/tunneld ./cmd/tunneld/main.go && \
    CGO_ENABLED=0 go build -o /out/tunnelctl ./cmd/tunnelctl/main.go

FROM alpine:3.20

ARG TARGETARCH
ARG KUBECTL_VERSION=v1.32.1

RUN apk add --no-cache \
    bash \
    bind-tools \
    ca-certificates \
    curl \
    iproute2 \
    netcat-openbsd \
    openssh-client \
    procps

RUN arch="${TARGETARCH:-amd64}"; \
    case "${arch}" in \
      amd64|arm64) ;; \
      *) echo "unsupported TARGETARCH=${arch}" >&2; exit 1 ;; \
    esac; \
    curl -fsSLo /usr/local/bin/kubectl "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/${arch}/kubectl"; \
    chmod +x /usr/local/bin/kubectl

COPY --from=build /out/tunneld /out/tunnelctl /usr/local/bin/

RUN mkdir -p /run/tunneld-it /var/lib/tunneld-it/keys /var/lib/tunneld-it/tunnels /tmp/tunneld-it
