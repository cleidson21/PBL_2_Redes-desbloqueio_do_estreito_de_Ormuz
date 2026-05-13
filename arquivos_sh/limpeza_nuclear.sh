docker stop $(docker ps -aq) 2>/dev/null; docker system prune -a --volumes -f
