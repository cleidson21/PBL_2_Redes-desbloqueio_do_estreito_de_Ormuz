#!/bin/bash

# Endereco do gateway na rede de testes.
IP_GATEWAY="172.16.103.8" 
QTD_SALAS=50

echo "🧠 Iniciando CÉREBROS (Clientes) de controle para $QTD_SALAS salas..."
echo "Alvo: $IP_GATEWAY"

for i in $(seq 1 $QTD_SALAS); do
    # Usa pseudo-terminal para manter a interface interativa funcionando.
    docker run -td --name "stress_cliente_$i" \
        -e INTEGRADOR_ADDR="$IP_GATEWAY:8083" \
        cleidsonramos/cliente:v2 > /dev/null
done

echo "✅ $QTD_SALAS Clientes ouvindo a rede e processando histerese!"
echo "💡 Dica: Para ver um painel funcionando, digite: docker attach stress_cliente_1"