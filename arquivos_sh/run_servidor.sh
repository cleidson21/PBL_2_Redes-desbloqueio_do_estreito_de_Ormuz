#!/bin/bash

set -e

echo "🚀 Iniciando o Servidor de Setor..."

# Remove a instancia antiga para evitar conflito de nome ao subir o servidor.
docker rm -f servidor_ormuz 2>/dev/null || true

# Sobe o servidor com as portas UDP e TCP usadas pela arquitetura atual.
docker build -t pbl-servidor ./servidor >/dev/null
docker run -d --name servidor_ormuz \
    -p 8080:8080/udp \
    -p 8081:8081/tcp \
    -p 8082:8082/tcp \
    -p 8083:8083/tcp \
    -p 8084:8084/tcp \
    -e MEU_SETOR=SETOR_A \
    pbl-servidor

echo "✅ Servidor iniciado com sucesso em background!"
echo "💡 Dica: Para acompanhar os logs em tempo real, digite: docker logs -f servidor_ormuz"