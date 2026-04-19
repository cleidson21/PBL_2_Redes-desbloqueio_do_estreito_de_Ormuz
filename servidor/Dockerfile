FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY main.go ./
RUN go build -o servidor main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/servidor .
# Expõe a porta UDP
EXPOSE 8080/udp
CMD ["./servidor"]
