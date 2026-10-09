# ---- build ----
FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/stoodify .

# ---- runtime ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=Asia/Jakarta
WORKDIR /app
COPY --from=builder /out/stoodify /app/stoodify
# Railway/Render menyuntikkan PORT; default 8080 untuk lokal.
EXPOSE 8080
CMD ["/app/stoodify"]
