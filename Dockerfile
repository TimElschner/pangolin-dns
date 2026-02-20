FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -o pangolin-dns .

FROM alpine:3.19
RUN apk add --no-cache wget
COPY --from=builder /app/pangolin-dns /usr/local/bin/
EXPOSE 53/udp 53/tcp 8080/tcp
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  CMD wget -qO- http://localhost:8080/healthz || exit 1
CMD ["pangolin-dns"]
