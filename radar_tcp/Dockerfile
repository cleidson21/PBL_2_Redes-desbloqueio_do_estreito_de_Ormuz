# Estágio de compilação
FROM golang:1.25-alpine AS builder
WORKDIR /app
# Copia os arquivos de módulo e o código
COPY go.mod ./
COPY main.go ./
# Compila o binário
RUN go build -o radar_tcp main.go

# Estágio final (imagem leve)
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/radar_tcp .
# Executa o sensor
CMD ["./radar_tcp"]
