FROM golang:1.26-alpine AS builder

WORKDIR /src

RUN apk add --no-cache ca-certificates git
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn

COPY backend/go.mod backend/go.sum ./backend/
WORKDIR /src/backend
RUN go mod download

COPY backend/ ./ 
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/api ./cmd/api

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata && adduser -D -g '' app

WORKDIR /app
COPY --from=builder /out/api /app/api
RUN mkdir -p /app/data /app/storage/uploads && chown -R app:app /app

USER app

EXPOSE 8080

ENTRYPOINT ["/app/api"]
