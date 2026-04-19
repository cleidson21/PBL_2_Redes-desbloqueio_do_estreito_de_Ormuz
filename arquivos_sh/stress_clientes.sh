#!/bin/bash

set -e

# Endereco do servidor na rede de testes.
IP_GATEWAY="${IP_GATEWAY:-172.16.103.8}"
QTD_SALAS="${QTD_SALAS:-50}"

echo "🧠 Iniciando dashboards de controle para $QTD_SALAS setores..."
echo "Alvo: $IP_GATEWAY"

docker build -t pbl-dashboard ./dashboard >/dev/null

for i in $(seq 1 $QTD_SALAS); do
    # Usa pseudo-terminal para manter a interface interativa funcionando.
    docker run -dit --name "stress_dashboard_$i" \
    -e SERVER_ADDR="$IP_GATEWAY:8083" \
        pbl-dashboard > /dev/null
done

echo "✅ $QTD_SALAS dashboards ouvindo a rede e processando telemetria!"
echo "💡 Dica: Para ver um painel funcionando, digite: docker attach stress_dashboard_1"