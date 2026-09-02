# Build a static go-trmnl binary and ship it on a minimal distroless image.
FROM golang:1.27 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    GOARM=$(echo "${TARGETVARIANT}" | tr -d 'v') \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /out/trmnld ./cmd/trmnld

# Pre-create the data dir so it is owned by the nonroot user in the final image.
RUN mkdir -p /data

# distroless/static includes CA certificates (needed for the weather plugin's
# HTTPS calls). Timezone data is embedded via time/tzdata in the binary.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/trmnld /usr/local/bin/trmnld
COPY --from=builder --chown=nonroot:nonroot /data /data

ENV TRMNL_LISTEN=":8080" \
    TRMNL_DATA_DIR="/data"
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/trmnld"]
