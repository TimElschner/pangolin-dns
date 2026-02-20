FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.* ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 go build -o pangolin-dns .

FROM alpine:3.19
COPY --from=builder /app/pangolin-dns /usr/local/bin/
EXPOSE 53/udp 53/tcp
CMD ["pangolin-dns"]
