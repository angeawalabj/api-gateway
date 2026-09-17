FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY main.go .
RUN go mod init upstream && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /upstream .

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /upstream /upstream
EXPOSE 8081
ENTRYPOINT ["/upstream"]
