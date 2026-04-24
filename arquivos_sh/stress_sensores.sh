#!/bin/bash

set -e

# Endereco do servidor na rede de testes.
IP_GATEWAY1="${IP_GATEWAY1:-172.16.201.5}"
IP_GATEWAY2="${IP_GATEWAY2:-172.16.201.6}"
IP_GATEWAY3="${IP_GATEWAY3:-172.16.201.7}"
IP_GATEWAY4="${IP_GATEWAY4:-172.16.201.8}"
QTD_SALAS="${QTD_SALAS:-3}"
IMG_SENSOR_TLM="${IMG_SENSOR_TLM:-cleidsonramos/sensor_tlm:latest}"
IMG_RADAR_TCP="${IMG_RADAR_TCP:-cleidsonramos/radar_tcp:latest}"

echo "📡 Iniciando tempestade de SENSORES para $QTD_SALAS setores..."
echo "Alvo: $IP_GATEWAY1, $IP_GATEWAY2, $IP_GATEWAY3, $IP_GATEWAY4"
echo "Imagens: $IMG_SENSOR_TLM | $IMG_RADAR_TCP"

docker pull "$IMG_SENSOR_TLM" >/dev/null
docker pull "$IMG_RADAR_TCP" >/dev/null

for i in $(seq 1 $QTD_SALAS); do
    # Sensor UDP de telemetria TLM.
    docker run -d --name "stress_sensor_tlm_$i" \
        -e SERVER_ADDRS="$IP_GATEWAY1:8080,$IP_GATEWAY2:8080,$IP_GATEWAY3:8080,$IP_GATEWAY4:8080" \
        -e SENSOR_ID="BOIA_$i" \
        "$IMG_SENSOR_TLM" > /dev/null

    # Sensor TCP de RADAR.
    docker run -d --name "stress_radar_tcp_$i" \
        -e SERVER_ADDRS="$IP_GATEWAY1:8081,$IP_GATEWAY2:8081,$IP_GATEWAY3:8081,$IP_GATEWAY4:8081" \
        -e SENSOR_ID="RADAR_$i" \
        -e SENSOR_TIPO="RADAR" \
        "$IMG_RADAR_TCP" > /dev/null

    # Sensor TCP de AIS (Sistema de Identificação Automática).
    docker run -d --name "stress_ais_tcp_$i" \
        -e SERVER_ADDRS="$IP_GATEWAY1:8081,$IP_GATEWAY2:8081,$IP_GATEWAY3:8081,$IP_GATEWAY4:8081" \
        -e SENSOR_ID="AIS_$i" \
        -e SENSOR_TIPO="AIS" \
        "$IMG_RADAR_TCP" > /dev/null

    # Sensor TCP de QUÍMICO.
    docker run -d --name "stress_quimico_tcp_$i" \
        -e SERVER_ADDRS="$IP_GATEWAY1:8081,$IP_GATEWAY2:8081,$IP_GATEWAY3:8081,$IP_GATEWAY4:8081" \
        -e SENSOR_ID="QUIMICO_$i" \
        -e SENSOR_TIPO="QUIMICO" \
        "$IMG_RADAR_TCP" > /dev/null
done

echo "✅ $QTD_SALAS Sensores TLM, $QTD_SALAS RADAR, $QTD_SALAS AIS e $QTD_SALAS QUÍMICO criados!"
