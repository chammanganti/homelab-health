FROM golang:1.25-alpine AS builder
WORKDIR /app

ARG TARGETARCH

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -ldflags="-w -s" -o homelab-health ./cmd/server

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /app/homelab-health /homelab-health
ENTRYPOINT ["/homelab-health"]
