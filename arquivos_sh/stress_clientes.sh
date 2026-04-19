#!/bin/bash

set -e

# Endereco do servidor na rede de testes.
IP_GATEWAY1="${IP_GATEWAY1:-172.16.103.8}"
IP_GATEWAY2="${IP_GATEWAY2:-172.16.103.9}"
IP_GATEWAY3="${IP_GATEWAY3:-172.16.103.10}"
QTD_SALAS="${QTD_SALAS:-50}"
IMG_DASHBOARD="${IMG_DASHBOARD:-cleidsonramos/dashboard:latest}"

echo "🧠 Iniciando dashboards de controle para $QTD_SALAS setores..."
echo "Alvo: $IP_GATEWAY1, $IP_GATEWAY2, $IP_GATEWAY3"
echo "Imagem: $IMG_DASHBOARD"

docker pull "$IMG_DASHBOARD" >/dev/null

for i in $(seq 1 $QTD_SALAS); do
    # Usa pseudo-terminal para manter a interface interativa funcionando.
    docker run -dit --name "stress_dashboard_$i" \
        -e SERVER_ADDRS="$IP_GATEWAY1:8083,$IP_GATEWAY2:8083,$IP_GATEWAY3:8083" \
        "$IMG_DASHBOARD" > /dev/null
done

echo "✅ $QTD_SALAS dashboards ouvindo a rede e processando telemetria!"
echo "💡 Dica: Para ver um painel funcionando, digite: docker attach stress_dashboard_1"