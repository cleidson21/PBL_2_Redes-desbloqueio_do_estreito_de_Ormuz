# Estágio de compilação
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY main.go ./
RUN go build -o drone main.go

# Estágio final
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/drone .

CMD ["./drone"]