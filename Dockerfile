FROM golang:1.24-alpine AS build

WORKDIR /src
RUN apk add --no-cache build-base
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -trimpath -ldflags="-s -w" -o /out/3do ./cmd/3do

FROM alpine:3.22

RUN addgroup -S app && adduser -S app -G app
WORKDIR /app
COPY --from=build /out/3do /app/3do
COPY web /app/web
RUN mkdir -p /data && chown -R app:app /data /app

USER app
ENV ADDR=:8080 DATA_DIR=/data
EXPOSE 8080
ENTRYPOINT ["/app/3do"]
