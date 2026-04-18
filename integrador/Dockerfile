FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod ./
COPY main.go ./
RUN go build -o integrador main.go

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/integrador .
# Expõe a porta UDP
EXPOSE 8080/udp
CMD ["./integrador"]
