FROM golang:1.24 AS builder
WORKDIR /src

ARG GOPROXY=https://goproxy.cn,direct
ARG GOSUMDB=sum.golang.google.cn
ENV GOPROXY=${GOPROXY}
ENV GOSUMDB=${GOSUMDB}

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/rss-scheduler ./cmd/rss-scheduler

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=builder /out/rss-scheduler /app/rss-scheduler
COPY --from=builder /src/configs /app/configs
ENTRYPOINT ["/app/rss-scheduler"]
