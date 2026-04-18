#!/bin/bash

# Endereco do gateway na rede de testes.
IP_GATEWAY="172.16.103.8" 
QTD_SALAS=50

echo "📡 Iniciando tempestade de SENSORES para $QTD_SALAS salas..."
echo "Alvo: $IP_GATEWAY"

for i in $(seq 1 $QTD_SALAS); do
    # Sensor UDP de temperatura.
    docker run -d --name "stress_sensor_udp_$i" \
        -e SERVER_ADDR="$IP_GATEWAY:8080" \
        -e SENSOR_ID="SALA_$i" \
        -e SENSOR_TIPO="T" \
        cleidsonramos/sensor_udp:v2 > /dev/null

    # Sensor TCP de eventos de acesso.
    docker run -d --name "stress_sensor_tcp_$i" \
        -e SERVER_ADDR="$IP_GATEWAY:8081" \
        -e SENSOR_ID="ENTRADA_$i" \
        -e SENSOR_TIPO="NFC" \
        cleidsonramos/sensor_tcp:v2 > /dev/null
done

echo "✅ $QTD_SALAS Sensores UDP e $QTD_SALAS Sensores TCP criados!"
