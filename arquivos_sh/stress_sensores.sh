#!/bin/bash

set -e

IP_GATEWAY1="${IP_GATEWAY1:-172.16.103.7}"
IP_GATEWAY2="${IP_GATEWAY2:-172.16.103.8}"
IP_GATEWAY3="${IP_GATEWAY3:-172.16.103.9}"
IP_GATEWAY4="${IP_GATEWAY4:-172.16.103.10}"
QTD_SALAS="${QTD_SALAS:-3}"
IMG_SENSOR_TLM="${IMG_SENSOR_TLM:-cleidsonramos/sensor_tlm:latest}"
IMG_RADAR_TCP="${IMG_RADAR_TCP:-cleidsonramos/radar_tcp:latest}"

echo "📡 Iniciando tempestade de SENSORES para $QTD_SALAS setores..."
echo "Alvo: $IP_GATEWAY1, $IP_GATEWAY2, $IP_GATEWAY3, $IP_GATEWAY4"
echo "Imagens: $IMG_SENSOR_TLM | $IMG_RADAR_TCP"

docker pull "$IMG_SENSOR_TLM" >/dev/null
docker pull "$IMG_RADAR_TCP" >/dev/null

for i in $(seq 1 $QTD_SALAS); do
    HEX_I=$(printf '%02X' $i)
    MAC_ADDR=$(printf '02:%02X:%02X:%02X:%02X:%s' $((RANDOM%256)) $((RANDOM%256)) $((RANDOM%256)) $((RANDOM%256)) "$HEX_I")
    MAC_TAG=${MAC_ADDR//:/}

    echo "  -> Subindo Sensores do setor $i com sufixo MAC: $MAC_ADDR"

    docker run -d --name "stress_sensor_tlm_${MAC_TAG}" \
        -e SERVER_ADDRS="$IP_GATEWAY1:48080,$IP_GATEWAY2:48080,$IP_GATEWAY3:48080,$IP_GATEWAY4:48080" \
        -e SENSOR_ID="BOIA_${MAC_ADDR}" \
        "$IMG_SENSOR_TLM" > /dev/null

    docker run -d --name "stress_radar_tcp_${MAC_TAG}" \
        -e SERVER_ADDRS="$IP_GATEWAY1:48081,$IP_GATEWAY2:48081,$IP_GATEWAY3:48081,$IP_GATEWAY4:48081" \
        -e SENSOR_ID="RADAR_${MAC_ADDR}" \
        -e SENSOR_TIPO="RADAR" \
        "$IMG_RADAR_TCP" > /dev/null

    docker run -d --name "stress_ais_tcp_${MAC_TAG}" \
        -e SERVER_ADDRS="$IP_GATEWAY1:48081,$IP_GATEWAY2:48081,$IP_GATEWAY3:48081,$IP_GATEWAY4:48081" \
        -e SENSOR_ID="AIS_${MAC_ADDR}" \
        -e SENSOR_TIPO="AIS" \
        "$IMG_RADAR_TCP" > /dev/null

    docker run -d --name "stress_quimico_tcp_${MAC_TAG}" \
        -e SERVER_ADDRS="$IP_GATEWAY1:48081,$IP_GATEWAY2:48081,$IP_GATEWAY3:48081,$IP_GATEWAY4:48081" \
        -e SENSOR_ID="QUIMICO_${MAC_ADDR}" \
        -e SENSOR_TIPO="QUIMICO" \
        "$IMG_RADAR_TCP" > /dev/null
done

echo "✅ $QTD_SALAS Sensores TLM, $QTD_SALAS RADAR, $QTD_SALAS AIS e $QTD_SALAS QUÍMICO criados!"
