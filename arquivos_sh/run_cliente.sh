#!/bin/bash

set -e

IP_GATEWAY1="${IP_GATEWAY1:-172.16.103.7}"
IP_GATEWAY2="${IP_GATEWAY2:-172.16.103.8}"
IP_GATEWAY3="${IP_GATEWAY3:-172.16.103.9}"
IP_GATEWAY4="${IP_GATEWAY4:-172.16.103.10}"
IMG_DASHBOARD="${IMG_DASHBOARD:-cleidsonramos/dashboard:latest}"

MAC_ADDR=$(printf '02:%02X:%02X:%02X:%02X:%02X' $((RANDOM%256)) $((RANDOM%256)) $((RANDOM%256)) $((RANDOM%256)) $((RANDOM%256)))
DASH_ID="DASH_${MAC_ADDR}"

echo "🧠 Iniciando Centro de Comando (Dashboard)..."
echo "Alvo: $IP_GATEWAY1, $IP_GATEWAY2, $IP_GATEWAY3, $IP_GATEWAY4"
echo "Imagem: $IMG_DASHBOARD"
echo "Identidade Única (MAC): $DASH_ID"

docker pull "$IMG_DASHBOARD" >/dev/null

docker run -dit --name "dashboard_ormuz" \
-e SERVER_ADDRS="$IP_GATEWAY1:48083,$IP_GATEWAY2:48083,$IP_GATEWAY3:48083,$IP_GATEWAY4:48083" \
-e DASHBOARD_ID="$DASH_ID" \
"$IMG_DASHBOARD" > /dev/null

echo "✅ Dashboard iniciado! Para acessar a interface digite: docker attach dashboard_ormuz"