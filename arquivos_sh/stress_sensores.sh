#!/bin/bash

set -e

# Endereco do servidor na rede de testes.
IP_GATEWAY1="${IP_GATEWAY1:-172.16.103.8}"
IP_GATEWAY2="${IP_GATEWAY2:-172.16.103.9}"
IP_GATEWAY3="${IP_GATEWAY3:-172.16.103.10}"
QTD_SALAS="${QTD_SALAS:-50}"
IMG_SENSOR_TLM="${IMG_SENSOR_TLM:-cleidsonramos/sensor_tlm:latest}"
IMG_RADAR_TCP="${IMG_RADAR_TCP:-cleidsonramos/radar_tcp:latest}"

echo "📡 Iniciando tempestade de SENSORES para $QTD_SALAS setores..."
echo "Alvo: $IP_GATEWAY1, $IP_GATEWAY2, $IP_GATEWAY3"
echo "Imagens: $IMG_SENSOR_TLM | $IMG_RADAR_TCP"

docker pull "$IMG_SENSOR_TLM" >/dev/null
docker pull "$IMG_RADAR_TCP" >/dev/null

for i in $(seq 1 $QTD_SALAS); do
    # Sensor UDP de telemetria.
    docker run -d --name "stress_sensor_tlm_$i" \
        -e SERVER_ADDRS="$IP_GATEWAY1:8080,$IP_GATEWAY2:8080,$IP_GATEWAY3:8080" \
        -e SENSOR_ID="BOIA_$i" \
        "$IMG_SENSOR_TLM" > /dev/null

    # Sensor TCP de radar crítico.
    docker run -d --name "stress_radar_tcp_$i" \
        -e SERVER_ADDRS="$IP_GATEWAY1:8081,$IP_GATEWAY2:8081,$IP_GATEWAY3:8081" \
        -e SENSOR_ID="RADAR_$i" \
        -e SENSOR_TIPO="RADAR" \
        "$IMG_RADAR_TCP" > /dev/null
done

echo "✅ $QTD_SALAS Sensores de telemetria e $QTD_SALAS radares criados!"
