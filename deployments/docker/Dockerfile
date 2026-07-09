## Build stage
FROM golang:1.22-bookworm AS build
WORKDIR /src
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/aegis ./cmd/aegis

## Runtime stage — distroless-style minimal image
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/aegis /app/aegis
COPY config.yaml /app/config.yaml
USER nonroot:nonroot
EXPOSE 50051 9090
ENTRYPOINT ["/app/aegis"]
CMD ["-config", "/app/config.yaml"]
