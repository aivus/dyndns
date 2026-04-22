FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o dyndns ./cmd/dyndns

FROM scratch

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/dyndns /dyndns

EXPOSE 8080

ENTRYPOINT ["/dyndns"]
CMD ["-config", "/config/config.yaml", "-addr", ":8080"]
