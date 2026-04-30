#!/bin/bash

set -e

# Endereco do servidor na rede de testes.
IP_GATEWAY1="${IP_GATEWAY1:-172.16.103.11}"
IP_GATEWAY2="${IP_GATEWAY2:-172.16.103.12}"
IP_GATEWAY3="${IP_GATEWAY3:-172.16.103.10}"
QTD_SALAS="${QTD_SALAS:-1}"
IMG_DASHBOARD="${IMG_DASHBOARD:-cleidsonramos/dashboard:latest}"

echo "🧠 Iniciando dashboards de controle para $QTD_SALAS setores..."
echo "Alvo: $IP_GATEWAY1, $IP_GATEWAY2, $IP_GATEWAY3"
echo "Imagem: $IMG_DASHBOARD"

docker pull "$IMG_DASHBOARD" >/dev/null

# Usa pseudo-terminal para manter a interface interativa funcionando.
docker run -dit --name "stress_dashboard_1" \
-e SERVER_ADDRS="$IP_GATEWAY1:8083,$IP_GATEWAY2:8083,$IP_GATEWAY3:8083" \
"$IMG_DASHBOARD" > /dev/null

