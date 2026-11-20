# ---- build ----
FROM golang:1.27-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Info build dipakai `stoodify version` (penting saat insiden: pastikan SHA yang jalan)
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" \
    -o /out/stoodify .

# ---- runtime ----
FROM alpine:3.20
# postgresql-client: dibutuhkan `stoodify db:backup` / `db:restore` / `rollback`
# (pg_dump & psql). gzip sudah tersedia lewat busybox.
RUN apk add --no-cache ca-certificates tzdata postgresql16-client
ENV TZ=Asia/Jakarta
WORKDIR /app
COPY --from=builder /out/stoodify /app/stoodify
# Railway/Render menyuntikkan PORT; default 8080 untuk lokal.
EXPOSE 8080
CMD ["/app/stoodify"]
