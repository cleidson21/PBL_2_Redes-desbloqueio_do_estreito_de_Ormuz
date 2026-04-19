#!/bin/bash

set -e

# Endereco do servidor na rede de testes.
IP_GATEWAY="${IP_GATEWAY:-172.16.103.8}"
QTD_SALAS="${QTD_SALAS:-50}"

echo "⚙️ Iniciando frota de DRONES para $QTD_SALAS setores..."
echo "Alvo: $IP_GATEWAY"

docker build -t pbl-drone ./drone >/dev/null

for i in $(seq 1 $QTD_SALAS); do
    # Drone para cada setor.
    docker run -d --name "stress_drone_$i" \
        -e SERVER_ADDR="$IP_GATEWAY:8082" \
        -e DRONE_ID="DRONE_$i" \
        pbl-drone > /dev/null
done

echo "✅ $QTD_SALAS drones conectados e registrados!"