#!/bin/bash
echo "🧹 Limpando containers de teste..."
# Remove apenas containers de stress criados para os testes.
CONTAINERS=$(docker ps -a -q --filter "name=stress_")
if [ -z "$CONTAINERS" ]; then
    echo "Nenhum container encontrado."
else
    docker rm -f $CONTAINERS
    echo "✅ Tudo limpo!"
fi