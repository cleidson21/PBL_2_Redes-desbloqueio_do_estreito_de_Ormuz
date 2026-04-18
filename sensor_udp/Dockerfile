# Estágio de compilação
FROM golang:1.25-alpine AS builder
WORKDIR /app
# Copia os arquivos de módulo e o código
COPY go.mod ./
COPY main.go ./
# Compila o binário
RUN go build -o sensor_udp main.go

# Estágio final (imagem leve)
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/sensor_udp .
# Executa o sensor
CMD ["./sensor_udp"]
