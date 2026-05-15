#!/bin/bash

set -e

IP_GATEWAY1="${IP_GATEWAY1:-172.16.103.7}"
IP_GATEWAY2="${IP_GATEWAY2:-172.16.103.8}"
IP_GATEWAY3="${IP_GATEWAY3:-172.16.103.9}"
IP_GATEWAY4="${IP_GATEWAY4:-172.16.103.10}"
QTD_SALAS="${QTD_SALAS:-5}"
IMG_DRONE="${IMG_DRONE:-cleidsonramos/drone:latest}"

echo "⚙️ Iniciando frota de DRONES para $QTD_SALAS setores..."
echo "Alvo: $IP_GATEWAY1, $IP_GATEWAY2, $IP_GATEWAY3, $IP_GATEWAY4"
echo "Imagem: $IMG_DRONE"

docker pull "$IMG_DRONE" >/dev/null

for i in $(seq 1 "$QTD_SALAS"); do
    HEX_I=$(printf '%02X' $i)
    MAC_ADDR=$(printf '02:%02X:%02X:%02X:%02X:%s' $((RANDOM%256)) $((RANDOM%256)) $((RANDOM%256)) $((RANDOM%256)) "$HEX_I")
    MAC_TAG=${MAC_ADDR//:/}
    echo "  -> Subindo Drone com MAC: $MAC_ADDR"
    docker run -d --name "stress_drone_${MAC_TAG}" \
        -e SERVER_ADDRS="$IP_GATEWAY1:48082,$IP_GATEWAY2:48082,$IP_GATEWAY3:48082,$IP_GATEWAY4:48082" \
        -e DRONE_ID="$MAC_ADDR" \
        "$IMG_DRONE" > /dev/null
done

echo "✅ $QTD_SALAS drones conectados e registrados!"
