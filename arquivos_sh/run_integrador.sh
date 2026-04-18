#!/bin/bash

echo "🚀 Iniciando o Integrador Gateway..."

# Remove a instancia antiga para evitar conflito de nome ao subir o gateway.
docker rm -f integrador_pbl 2>/dev/null

# Sobe o integrador com as portas UDP e TCP usadas pela arquitetura.
docker run -d --name integrador_pbl \
    -p 8080:8080/udp \
    -p 8081:8081/tcp \
    -p 8082:8082/tcp \
    -p 8083:8083/tcp \
    cleidsonramos/integrador:v2

echo "✅ Integrador iniciado com sucesso em background!"
echo "💡 Dica: Para acompanhar os logs em tempo real, digite: docker logs -f integrador_pbl"