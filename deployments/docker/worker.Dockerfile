FROM golang:1.24 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/rss-worker ./cmd/rss-worker

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /out/rss-worker /app/rss-worker
COPY --from=builder /src/configs /app/configs
ENTRYPOINT ["/app/rss-worker"]
