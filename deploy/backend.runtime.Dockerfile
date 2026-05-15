FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata wget && adduser -D -g '' app

WORKDIR /app
COPY backend/api /app/api
RUN chmod +x /app/api && mkdir -p /app/data /app/storage/uploads && chown -R app:app /app

USER app

EXPOSE 8080

ENTRYPOINT ["/app/api"]
