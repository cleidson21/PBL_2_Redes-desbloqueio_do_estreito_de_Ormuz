#!/bin/bash

set -e

# Endereco do servidor na rede de testes.
IP_GATEWAY1="${IP_GATEWAY1:-172.16.103.7}"
IP_GATEWAY2="${IP_GATEWAY2:-172.16.103.8}"
IP_GATEWAY3="${IP_GATEWAY3:-172.16.103.9}"
IP_GATEWAY4="${IP_GATEWAY4:-172.16.103.10}"
IMG_DASHBOARD="${IMG_DASHBOARD:-cleidsonramos/dashboard:latest}"

# Gera um MAC Address pseudo-aleatório válido para o Dashboard
MAC_ADDR=$(printf '02:%02X:%02X:%02X:%02X:%02X' $((RANDOM%256)) $((RANDOM%256)) $((RANDOM%256)) $((RANDOM%256)) $((RANDOM%256)))
DASH_ID="DASH_${MAC_ADDR}"

echo "🧠 Iniciando Centro de Comando (Dashboard)..."
echo "Alvo: $IP_GATEWAY1, $IP_GATEWAY2, $IP_GATEWAY3, $IP_GATEWAY4"
echo "Imagem: $IMG_DASHBOARD"
echo "Identidade Única (MAC): $DASH_ID"

docker pull "$IMG_DASHBOARD" >/dev/null

# Usa pseudo-terminal para manter a interface interativa funcionando.
# Passamos a variavel DASHBOARD_ID (ou CLIENT_ID, dependendo de como está no seu main.go do cliente)
docker run -dit --name "dashboard_ormuz" \
-e SERVER_ADDRS="$IP_GATEWAY1:8083,$IP_GATEWAY2:8083,$IP_GATEWAY3:8083,$IP_GATEWAY4:8083" \
-e DASHBOARD_ID="$DASH_ID" \
"$IMG_DASHBOARD" > /dev/null

echo "✅ Dashboard iniciado! Para acessar a interface digite: docker attach dashboard_ormuz"