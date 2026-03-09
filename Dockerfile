FROM golang:1.23.0 AS builder
WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o rate-limiter ./cmd/server

FROM debian:12-slim
WORKDIR /app
# Copy the built binary
COPY --from=builder /app/rate-limiter /app/rate-limiter
# Copy the config file to the
COPY --from=builder /app/config /app/config
# Copy scripts
COPY --from=builder /app/scripts /app/scripts

EXPOSE 8080
CMD ["/app/rate-limiter"]
