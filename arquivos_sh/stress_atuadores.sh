#!/bin/bash

# Endereco do gateway na rede de testes.
IP_GATEWAY="172.16.103.8" 
QTD_SALAS=50

echo "⚙️ Iniciando exército de ATUADORES para $QTD_SALAS salas..."
echo "Alvo: $IP_GATEWAY"

for i in $(seq 1 $QTD_SALAS); do
    # Atuador de ar-condicionado para cada sala.
    docker run -d --name "stress_atuador_ac_$i" \
        -e INTEGRADOR_ADDR="$IP_GATEWAY:8082" \
        -e ATUADOR_ID="SALA_$i" \
        -e ATUADOR_TIPO="AC" \
        cleidsonramos/atuador_ac:v2 > /dev/null

    # Atuador de lampada para cada sala.
    docker run -d --name "stress_atuador_led_$i" \
        -e INTEGRADOR_ADDR="$IP_GATEWAY:8082" \
        -e ATUADOR_ID="SALA_$i" \
        -e ATUADOR_TIPO="LED" \
        cleidsonramos/atuador_led:v2 > /dev/null
done

echo "✅ $QTD_SALAS Atuadores AC e $QTD_SALAS Lâmpadas conectados e registrados!"