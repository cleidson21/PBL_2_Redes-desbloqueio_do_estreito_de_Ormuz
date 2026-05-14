#!/bin/bash
echo "🧹 Limpando containers de teste..."
CONTAINERS=$(docker ps -a -q --filter "name=stress_")
if [ -z "$CONTAINERS" ]; then
    echo "Nenhum container encontrado."
else
    docker rm -f $CONTAINERS
    echo "✅ Tudo limpo!"
fi