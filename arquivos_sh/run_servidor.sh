#!/bin/bash

set -e

echo "🚀 Iniciando o Servidor de Setor..."
NOME_SETOR="${NOME_SETOR:-SETOR_NORTE}"
IMAGE_SERVIDOR="${IMAGE_SERVIDOR:-cleidsonramos/servidor:latest}"
USE_LOCAL_BUILD="${USE_LOCAL_BUILD:-false}"
# Remove a instancia antiga para evitar conflito de nome ao subir o servidor.
docker rm -f servidor_ormuz 2>/dev/null || true

# Define a imagem a usar (Docker Hub por padrão, build local opcional).
if [[ "$USE_LOCAL_BUILD" == "true" ]]; then
    IMAGE_SERVIDOR="pbl-servidor:local"
    docker build -t "$IMAGE_SERVIDOR" ./servidor >/dev/null
else
    docker pull "$IMAGE_SERVIDOR" >/dev/null
fi

# Sobe o servidor com as portas UDP e TCP usadas pela arquitetura atual.
docker run -d --name servidor_ormuz \
    -p 8080:8080/udp \
    -p 8081:8081/tcp \
    -p 8082:8082/tcp \
    -p 8083:8083/tcp \
    -p 8084:8084/tcp \
    -e MEU_SETOR="$NOME_SETOR" \
    "$IMAGE_SERVIDOR"

echo "✅ Servidor iniciado com sucesso em background!"
echo "💡 Dica: Para acompanhar os logs em tempo real, digite: docker logs -f servidor_ormuz"