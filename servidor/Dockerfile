FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY *.go ./
RUN go build -o servidor .

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/servidor .
# Expõe as portas usadas pelo setor
EXPOSE 8080/udp
EXPOSE 8081/tcp
EXPOSE 8082/tcp
EXPOSE 8083/tcp
EXPOSE 8084/tcp
CMD ["./servidor"]
