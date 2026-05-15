FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY *.go ./
RUN go build -o servidor .

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/servidor .
# Expõe as portas usadas pelo setor (ajustadas para ambiente de laboratório)
EXPOSE 48080/udp
EXPOSE 48081/tcp
EXPOSE 48082/tcp
EXPOSE 48083/tcp
EXPOSE 48084/tcp
CMD ["./servidor"]
