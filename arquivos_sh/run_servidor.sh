#!/bin/bash

set -e

echo "🚀 Iniciando o Servidor de Setor..."

IP_PREFIX="${IP_PREFIX:-172.16.103}"
IP_INICIO="${IP_INICIO:-1}"
IP_FIM="${IP_FIM:-16}"

obter_octeto_local() {
    local ip_local

    if [[ -n "${HOST_OCTET:-}" ]]; then
        echo "$HOST_OCTET"
        return 0
    fi

    ip_local="$(hostname -I 2>/dev/null | tr ' ' '\n' | grep -E "^${IP_PREFIX//./\\.}\\.[0-9]+$" | head -n 1 || true)"
    if [[ -z "$ip_local" ]]; then
        return 1
    fi

    echo "${ip_local##*.}"
}

HOST_OCTET="$(obter_octeto_local || true)"

if [[ -z "$HOST_OCTET" ]]; then
    echo "❌ Não foi possível detectar o IP local dentro da faixa ${IP_PREFIX}.1-${IP_PREFIX}.${IP_FIM}."
    echo "💡 Defina HOST_OCTET manualmente, por exemplo: HOST_OCTET=3 ./arquivos_sh/run_servidor.sh"
    exit 1
fi

NOME_SETOR="${NOME_SETOR:-SETOR_$(printf '%02d' "$HOST_OCTET")}"

PEERS_LIST=()
for i in $(seq "$IP_INICIO" "$IP_FIM"); do
    if [[ "$i" == "$HOST_OCTET" ]]; then
        continue
    fi
    PEERS_LIST+=("${IP_PREFIX}.${i}:48084")
done

PEERS="${PEERS:-$(IFS=,; echo "${PEERS_LIST[*]}")}"

IMAGE_SERVIDOR="${IMAGE_SERVIDOR:-cleidsonramos/servidor:latest}"
USE_LOCAL_BUILD="${USE_LOCAL_BUILD:-false}"

docker rm -f servidor_ormuz 2>/dev/null || true

if [[ "$USE_LOCAL_BUILD" == "true" ]]; then
    IMAGE_SERVIDOR="pbl-servidor:local"
    docker build -t "$IMAGE_SERVIDOR" ./servidor >/dev/null
else
    docker pull "$IMAGE_SERVIDOR" >/dev/null
fi

docker run -d --name servidor_ormuz \
    -p 48080:48080/udp \
    -p 48081:48081/tcp \
    -p 48082:48082/tcp \
    -p 48083:48083/tcp \
    -p 48084:48084/tcp \
    -e MEU_SETOR="$NOME_SETOR" \
    -e PEERS="$PEERS" \
    "$IMAGE_SERVIDOR"

echo "✅ Servidor [$NOME_SETOR] iniciado com sucesso em background!"
echo "💡 Dica: Para acompanhar os logs em tempo real, digite: docker logs -f servidor_ormuz"
