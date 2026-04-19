#!/bin/bash

set -e

# Endereco do servidor na rede de testes.
IP_GATEWAY="${IP_GATEWAY:-172.16.103.8}"
QTD_SALAS="${QTD_SALAS:-50}"

echo "📡 Iniciando tempestade de SENSORES para $QTD_SALAS setores..."
echo "Alvo: $IP_GATEWAY"

docker build -t pbl-sensor-tlm ./sensor_tlm >/dev/null
docker build -t pbl-radar-tcp ./radar_tcp >/dev/null

for i in $(seq 1 $QTD_SALAS); do
    # Sensor UDP de telemetria.
    docker run -d --name "stress_sensor_tlm_$i" \
        -e SERVER_ADDR="$IP_GATEWAY:8080" \
        -e SENSOR_ID="BOIA_$i" \
        pbl-sensor-tlm > /dev/null

    # Sensor TCP de radar crítico.
    docker run -d --name "stress_radar_tcp_$i" \
        -e SERVER_ADDR="$IP_GATEWAY:8081" \
        -e SENSOR_ID="RADAR_$i" \
        -e SENSOR_TIPO="RADAR" \
        pbl-radar-tcp > /dev/null
done

echo "✅ $QTD_SALAS Sensores de telemetria e $QTD_SALAS radares criados!"
