#!/bin/bash

set -e

# Endereco do servidor na rede de testes.
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
    # 1. Converte o número do loop para Hexadecimal (ex: 1 vira 01, 10 vira 0A)
    HEX_I=$(printf '%02X' $i)
    # 2. Gera um MAC Address pseudo-aleatório válido.
    MAC_ADDR=$(printf '02:%02X:%02X:%02X:%02X:%s' $((RANDOM%256)) $((RANDOM%256)) $((RANDOM%256)) $((RANDOM%256)) "$HEX_I")
    echo "  -> Subindo Drone com MAC: $MAC_ADDR"
    # 3. Injeta o MAC Address como a identidade do Drone
    docker run -d --name "stress_drone_$i" \
        -e SERVER_ADDRS="$IP_GATEWAY1:8082,$IP_GATEWAY2:8082,$IP_GATEWAY3:8082,$IP_GATEWAY4:8082" \
        -e DRONE_ID="$MAC_ADDR" \
        "$IMG_DRONE" > /dev/null
done

echo "✅ $QTD_SALAS drones conectados e registrados!"
