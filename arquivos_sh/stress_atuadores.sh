#!/bin/bash

set -e

# Endereco do servidor na rede de testes.
IP_GATEWAY1="${IP_GATEWAY1:-172.16.103.8}"
IP_GATEWAY2="${IP_GATEWAY2:-172.16.103.9}"
IP_GATEWAY3="${IP_GATEWAY3:-172.16.103.10}"
QTD_SALAS="${QTD_SALAS:-50}"
IMG_DRONE="${IMG_DRONE:-cleidsonramos/drone:latest}"

echo "⚙️ Iniciando frota de DRONES para $QTD_SALAS setores..."
echo "Alvo: $IP_GATEWAY1, $IP_GATEWAY2, $IP_GATEWAY3"
echo "Imagem: $IMG_DRONE"

docker pull "$IMG_DRONE" >/dev/null

for i in $(seq 1 $QTD_SALAS); do
    # Drone para cada setor.
    docker run -d --name "stress_drone_$i" \
        -e SERVER_ADDRS="$IP_GATEWAY1:8082,$IP_GATEWAY2:8082,$IP_GATEWAY3:8082" \
        -e DRONE_ID="DRONE_$i" \
        "$IMG_DRONE" > /dev/null
done

echo "✅ $QTD_SALAS drones conectados e registrados!"