FROM golang:1.24-alpine AS build

WORKDIR /src
RUN apk add --no-cache build-base
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/3do ./cmd/3do

FROM alpine:3.22

RUN apk add --no-cache su-exec \
	&& addgroup -S -g 1000 app \
	&& adduser -S -D -H -u 1000 -G app app
WORKDIR /app
COPY --from=build /out/3do /app/3do
COPY web /app/web
COPY scripts/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh \
	&& mkdir -p /data \
	&& chown -R app:app /data

ENV PORT=8080 DATA_DIR=/data PUID=1000 PGID=1000
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["/app/3do"]
