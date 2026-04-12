FROM node:22-alpine AS web-builder
WORKDIR /src/web

COPY web/package.json web/package-lock.json ./
RUN npm ci

COPY web .
RUN npm run build

FROM golang:1.24 AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/rss-api ./cmd/rss-api

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /out/rss-api /app/rss-api
COPY --from=builder /src/configs /app/configs
COPY --from=builder /src/migrations /app/migrations
COPY --from=web-builder /src/web/dist /app/web/dist
ENV APP_STATIC_DIR=/app/web/dist
EXPOSE 8080
ENTRYPOINT ["/app/rss-api"]
