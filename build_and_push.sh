#!/bin/bash

set -euo pipefail

# Define o seu utilizador do Docker Hub
DOCKER_USER="cleidsonramos"

echo "🚀 A iniciar Build e Push para o Docker Hub ($DOCKER_USER)..."

# Array com os nomes dos serviços/pastas
SERVICOS=("servidor" "dashboard" "drone" "radar_tcp" "sensor_tlm")

for SERVICO in "${SERVICOS[@]}"; do
    echo "---------------------------------------------------"
    echo "📦 A construir imagem: $DOCKER_USER/$SERVICO:latest"
    docker build -t "$DOCKER_USER/$SERVICO:latest" "./$SERVICO"
    
    echo "☁️  A enviar $SERVICO para o Docker Hub..."
    docker push "$DOCKER_USER/$SERVICO:latest"
done

echo "---------------------------------------------------"
echo "✅ Todas as imagens foram atualizadas no Docker Hub com sucesso!"