# GUIA DE TESTES - Sistema de Filas Modularizado

> **CENÁRIOS REAIS com 2+ MÁQUINAS (172.16.201.0/24)**
> 
> Este guia apresenta testes práticos com a rede de laboratório, usando scripts de stress (`arquivos_sh/`) com quantidades predefinidas de sensores e drones.

---

## 1️⃣ CENÁRIO BASELINE: 2 SERVIDORES + 1 SENSOR + 1 DRONE

### 📋 Objetivo
Validar que um servidor consegue receber registros de sensor, enfileirar alertas, despachar drone e sincronizar com servidor par.

### 🔧 Setup

**Máquina 1 (172.16.201.5):**
```bash
cd /home/cleidson/PBL_2_Redes-desbloqueio_do_estreito_de_Ormuz
docker-compose up servidor5 -d
sleep 2
# Logs: "Servidor iniciado em 172.16.201.5:48080"
```

**Máquina 2 (172.16.201.6):**
```bash
# Repetir com host 172.16.201.6 no docker-compose
docker-compose up servidor6 -d
# Logs: "Conectando a 172.16.201.5:9000 para P2P"
```

**Máquina 1 - Iniciar 1 sensor TLM:**
```bash
cd arquivos_sh
bash stress_sensores.sh 1  # QTD_SALAS=1
# Spawns: sensor_tlm_1 (UDP to 172.16.201.5:6000)
# Sleep 5s para calibração
```

**Máquina 1 - Iniciar 1 drone:**
```bash
bash stress_atuadores.sh 1  # QTD_SALAS=1
# Spawns: drone_1 registrado em 172.16.201.5:48080
# Initial GW: 172.16.201.5 (primário)
```

### ✅ Critérios de Sucesso

1. **Servidor 5 recebe registração de drone:**
   ```bash
   docker logs servidor5 | grep "DRONE_1 registrado"
   # Esperado: "[DRONE_1] registrado em SETOR_05"
   ```

2. **Sensor enfileira alerta em servidor 5:**
   ```bash
   docker logs servidor5 | grep "QUEUE STATUS"
   # Esperado: "QUEUE STATUS: 1 críticos, 0 normais"
   ```

3. **Servidor 5 despacha drone (Ricart/Lamport):**
   ```bash
   docker logs servidor5 | grep "CONSENSO_ALCANÇADO\|DESPACHO_CONFIRMADO"
   # Esperado: "CONSENSO_ALCANÇADO SETOR_06 (timestamp=42)"
   # Esperado: "DESPACHO_CONFIRMADO DRONE_1"
   ```

4. **Sincronização Gossip: Servidor 6 recebe atualizacao:**
   ```bash
   docker logs servidor6 | grep "SINCRONIZANDO\|FrotaGlobal"
   # Esperado: "SINCRONIZANDO com 172.16.201.5 (LamportClock=42)"
   ```

5. **E2E Latency < 2s:**
   ```bash
   timestamp_sensor=$(date +%s%N | cut -b1-13)  # Milliseconds
   # [Esperar alerta ser enfileirado e despachado]
   timestamp_despacho=$(grep "DESPACHO_CONFIRMADO" < docker logs servidor5 | tail -1)
   # Calcular diferença < 2000ms
   ```

### 🛑 Troubleshooting
- **Sensor não conecta:** `docker logs sensor_tlm_1 | grep "ERROR\|FAIL"`
- **Drone não registra:** `netstat -an | grep 172.16.201.5:48080`
- **Gossip falha:** `docker logs servidor6 | grep "P2P_ERROR\|SYNC_FAIL"`

---

## 2️⃣ CENÁRIO SCALE: 4 SERVIDORES + 3 SENSORES + 3 DRONES

### 📋 Objetivo
Validar consenso distribuído (Ricart-Agrawala), replicação Gossip e balanceamento de carga com quorum de 4.

### 🔧 Setup

**Iniciar 4 servidores (ex: máquinas diferentes ou containers em bridge network):**
```bash
# Máquina A: 172.16.201.5
HOST_OCTET=5 bash run_servidor.sh

# Máquina B: 172.16.201.6
HOST_OCTET=6 bash run_servidor.sh

# Máquina C: 172.16.201.7
HOST_OCTET=7 bash run_servidor.sh

# Máquina D: 172.16.201.8
HOST_OCTET=8 bash run_servidor.sh
```

**Iniciar sensores:**
```bash
# Qualquer máquina
QTD_SALAS=3 bash stress_sensores.sh 3
# Spawns:
# - sensor_tlm_1, sensor_tlm_2, sensor_tlm_3 (UDP)
# - radar_1, radar_2, radar_3 (TCP com retry a gateways)
# - ais_1, ais_2, ais_3 (TCP)
# - quimico_1, quimico_2, quimico_3 (TCP)
# Total: 12 conexões distribuídas entre .5:.8
```

**Iniciar drones:**
```bash
QTD_SALAS=3 bash stress_atuadores.sh 3
# Spawns: drone_1, drone_2, drone_3
# Cada um conhece gateways: [172.16.201.5, 172.16.201.6, 172.16.201.7, 172.16.201.8]
# Failover automático se gateway principal cair
```

### ✅ Critérios de Sucesso

1. **Todos 4 servidores conhecem-se (P2P Mesh):**
   ```bash
   docker logs servidor5 | grep "CONECTADO\|VIZINHOS"
   # Esperado: 3 conexões (para .6, .7, .8)
   ```

2. **Sensores distribuem-se entre servidores:**
   ```bash
   for i in 5 6 7 8; do
     echo "=== Servidor $i ==="; 
     docker logs servidor$i | grep "registrado em SETOR" | wc -l
   done
   # Esperado: ~3 sensores cada (distribuição ~igual)
   ```

3. **Consenso Ricart com 4 participantes:**
   ```bash
   docker logs servidor5 | grep "RICART_REQUEST\|CONSENSO_ALCANÇADO" | head -20
   # Esperado: cada despacho tem ACK de 3 vizinhos antes de ALCANÇADO
   ```

4. **Replicação Gossip propaga estado:**
   ```bash
   # Clone estado atual de SETOR_05
   ESTADO_05=$(docker logs servidor5 | grep "FrotaGlobal" | tail -1)
   sleep 2  # Aguardar gossip
   # Verificar se SETOR_06 tem mesmo estado
   docker logs servidor6 | grep "FrotaGlobal" | grep "$ESTADO_05"
   # Esperado: encontrado (replicação funcionou)
   ```

5. **Throughput > 100 alerts/min (3 sensores × 30+/min):**
   ```bash
   echo "Alertas últimos 60s em SETOR_05:"
   docker logs servidor5 --since 60s | grep "DESPACHO_CONFIRMADO" | wc -l
   # Esperado: >= 100
   ```

6. **E2E Latency < 3s tipicamente, <5s no p95:**
   ```bash
   # Ver logs de entrada vs saída
   docker logs servidor5 | grep "ENTRADA_SENSOR\|DESPACHO" | \
     awk '{print $1}' | paste - - | awk '{print $2-$1}' | sort -n | tail -5
   # Esperado: p95 < 5000ms
   ```

### 🛑 Troubleshooting
- **Consenso travado:** `docker logs servidor5 | grep "ESPERANDO\|TIMEOUT"`
- **Gossip desincronizado:** `docker logs servidor6 servidor7 | grep "LAMPORT.*MISMATCH\|SYNC_FAIL"`
- **Sensores desconectam:** Verificar se failover ativou: `docker logs sensor_tlm_1 | grep "TENTANDO_GATEWAY\|RECONECTANDO"`

---

## 3️⃣ CENÁRIO STRESS: 4 SERVIDORES + 20 SENSORES + 20 DRONES

### 📋 Objetivo
Validar performance sob carga máxima, CPU/Memória, starvation prevention da fila.

### 🔧 Setup

```bash
# Terminal 1: Servidores (reutilizar do Cenário Scale)
# Terminal 2: Stress sensores
QTD_SALAS=20 bash stress_sensores.sh 20
# Spawns 80 containers de sensor (20×4 tipos)
# Cada um dispara ~50 alerts/min = ~4000 alerts/min total

# Terminal 3: Stress drones
QTD_SALAS=20 bash stress_atuadores.sh 20
# Spawns 20 drones, cada um registrado em um dos 4 servidores

# Terminal 4: Monitorar CPU/Memória
watch -n 1 'docker stats servidor5 servidor6 servidor7 servidor8'
```

### ✅ Critérios de Sucesso

1. **Fila nunca estoura (maxSize=100):**
   ```bash
   docker logs servidor5 | grep "QUEUE_OVERFLOW\|DESCARTE"
   # Esperado: NENHUMA linha (fila processada in-time)
   ```

2. **Starvation Prevention ativo:**
   ```bash
   docker logs servidor5 | grep "PROMOVIDO_NORMAL_A_CRITICO"
   # Esperado: algumas linhas durante stress (~1-2 por minuto é normal)
   ```

3. **CPU < 70% por servidor:**
   ```bash
   docker stats --no-stream | grep servidor[5-8] | awk '{print $3}'
   # Esperado: <70% durante stress
   ```

4. **Memória estável (< 200MB por servidor):**
   ```bash
   docker stats --no-stream | grep servidor[5-8] | awk '{print $4}'
   # Esperado: <200MB, sem crescimento contínuo
   ```

5. **Latência sob stress < 8s no p99:**
   ```bash
   # Durante 1 min de stress, coletar latências
   docker logs servidor5 --since 60s | grep "LATENCIA\|DESPACHO_TIME" | \
     sed 's/.*latencia=//; s/ms.*//' | sort -n | tail -1
   # Esperado: <8000ms
   ```

### 🛑 Troubleshooting
- **OOM Killer ativa:** `dmesg | tail -20` (aumentar RAM ou reduzir QTD_SALAS)
- **Deadlock Ricart:** `docker logs servidor5 | grep -c "ESPERANDO"` > 100 por 30s = problema
- **Gossip lag crescente:** `docker logs | grep "LAMPORT_DIFF" | tail -10` (deve ser < 5)

---

## 4️⃣ CENÁRIO FAILOVER: 4 SERVIDORES → Kill 1 → Validar Reconexão

### 📋 Objetivo
Testar resiliência: clientes devem reconectar automaticamente ao servidor caído.

### 🔧 Setup

```bash
# Iniciar todos como no Cenário Scale
# ... (rodar Cenário 2 completo primeiro) ...

# Aguardar estabilidade (30s)
sleep 30

# Terminal 1: Começar log watch
for i in 5 6 7 8; do
  docker logs -f servidor$i 2>&1 | grep "RECONECTANDO\|REGISTRADO\|FALHA" &
done

# Terminal 2: Kill servidor .5
docker stop servidor5
timestamp_kill=$(date +%s)

# Observar reconexão
# Terminal 3: Checar logs de drones/sensores
for i in 1 2 3; do
  echo "=== sensor_tlm_$i ==="
  docker logs sensor_tlm_$i | tail -5
done
```

### ✅ Critérios de Sucesso

1. **Sensores reconectam em < 10s:**
   ```bash
   # Verificar primeira reconexão após kill
   grep -m1 "RECONECTANDO_A_NOVO_GATEWAY" < docker logs sensor_tlm_1
   # Esperado: timestamp_reconnect - timestamp_kill < 10000ms
   ```

2. **Drones reconectam em < 10s:**
   ```bash
   docker logs drone_1 | grep "RECONECTANDO\|NOVO_GATEWAY" | head -1
   # Esperado: encontrado
   ```

3. **Alertas continuam sendo processados por .6, .7, .8:**
   ```bash
   docker logs servidor6 | tail -100 | grep "DESPACHO_CONFIRMADO" | wc -l
   # Esperado: > 0 (processamento continua, mesmo sem .5)
   ```

4. **Quando .5 volta, gossip sincroniza:**
   ```bash
   docker start servidor5
   sleep 5
   docker logs servidor5 | grep "SINCRONIZANDO\|LAMPORT.*CATCH_UP"
   # Esperado: vê backlog de eventos de .6, .7, .8 durante offline
   ```

5. **No máximo 1 falha de despacho durante transitório:**
   ```bash
   docker logs | grep "DESPACHO_RECUSADO\|RICART_TIMEOUT" | grep -A5 -B5 "$timestamp_kill" | wc -l
   # Esperado: < 3 linhas (falhas mínimas)
   ```

### 🛑 Troubleshooting
- **Reconexão > 30s:** Aumentar timeout em `listeners.go` (padrão 5s)
- **Gossip não sincroniza:** Verificar se Lamport está sendo propagado: `docker logs servidor5 | grep "LAMPORT_CLOCK"`
- **Drones ficam órfãos:** Checar failover list em ricart.go (deve ter backup)

---

## 📊 RESUMO: ESCALAÇÃO RECOMENDADA

```
Baseline (2 srv) → Scale (4 srv) → Stress (4 srv + x20) → Failover
    ✅ Funciona         ✅ Consenso      ✅ Carga max     ✅ Resiliência
    P: básico          P: distribuído   P: throughput   P: tolerância
   E2E: <2s            E2E: <3s         E2E: <8s        E2E: <10s
   CPU: -              CPU: <50%        CPU: <70%       CPU: normal
```

---

## 🔍 MODO DEBUG: Capturando Tudo

```bash
# Terminal dedicado: streaming logs de todos os servidores
docker logs -f servidor5 servidor6 servidor7 servidor8 2>&1 | tee /tmp/all_servers.log

# Análise post-mortem
# Extrair timeline de evento crítico
grep "RICART_REQUEST\|ACK_RECEIVED\|CONSENSO_ALCANÇADO" /tmp/all_servers.log | \
  sed 's/^[^0-9]*//' | sort | head -20

# Verificar sincronização Lamport
grep "LAMPORT_CLOCK" /tmp/all_servers.log | tail -20 | awk '{print $NF}' | sort -u
# Esperado: drift < 5 entre servidores
```
}
```

---

### C. **lamport.go - Clock**

```go
// test_lamport_test.go
func TestLamportIncrement(t *testing.T) {
    gs := NewGlobalState("SETOR_06", 100, 3)
    
    r1 := TickLamport(gs)
    r2 := TickLamport(gs)
    
    if r2 != r1+1 {
        t.Fail()
    }
}

func TestLamportSync(t *testing.T) {
    gs := NewGlobalState("SETOR_06", 100, 3)
    
    SyncLamport(gs, 100)  // Recebe 100 de remoto
    
    if gs.Relogio != 101 { // 100 -> sync, then ++ ->101
        t.Fail()
    }
}
```

---

## 🔄 TESTES DE INTEGRAÇÃO

### Cenário 1: Alert normal → crítico com starvation

**Passos:**
1. Enqueue: 10 alertas normais + 20 críticos
2. Consumer consome critical
3. Após 3 críticos, 1 normal sobe para 2
4. Valida que log mostra "🚀 Starvation Prevention"

**Esperado:** Nenhum alerta perdido, promoção automática em tempo apropriado

---

### Cenário 2: TLM Threshold Trigger

**Setup:**
```bash
# Terminal 1
export MEU_SETOR=SETOR_06 PEERS=localhost:48084
./servidor_local

# Terminal 2
export SENSOR_ID=SENSOR_VENTO_01 SERVER_ADDRS=localhost:48080
./sensor_tlm_local

# Monitorar output: deve ver "🚨 [THRESHOLD ALERT]" quando valor > 70 por 2 leituras
```

**Esperado:** A cada ~4s (2x intervalo de 2s), sensor reporta quando valor > 70

---

### Cenário 3: Gossip propagação com nova queue

**Setup:**
```bash
# Simular 2 setor (em máquinas diferentes ou portas diferentes)
SETOR_06: :48084 (P2P), :48080 (UDP), :48081-83 (TCP)
SETOR_07: :9084 (P2P), :9080 (UDP), :9081-83 (TCP)
```

**Validação:**
1. Alerta chega em SETOR_06
2. Queue enfileira → Consumer executa Ricart
3. P2P_REQ sai para SETOR_07
4. SETOR_07 nega se em uso, enfileira se LIVRE
5. Se SETOR_07 ganha, dispatcher remoto ordena drone de SETOR_06

**Esperado:** Sem deadlock, ambos setores sincronizados em frota

---

## 📊 TESTE DE CARGA

### Stress Test: 100 Alertas em 1 Segundo

```bash
# Simular picos de radar (múltiplas conexões)
for i in {1..100}; do
    echo '{"tipo":"EVT","acao":"ALERTA","posicao":"40.2,-72.5"}' | \
   nc -w 1 localhost 48081 &
done
wait

# Validações:
# 1. Queue stats: devem mostrar até 100 normais + N críticos
# 2. Nenhum panic em servidor
# 3. Drones recebem CMDs ordenadamente
```

**Esperado:**
- CPU < 50%
- Memory estável (sem memory leak)
- Todos os alertas eventualmente processados
- Latência E2E < 5s

---

## 🔍 LOGS ESPERADOS

### Boot do servidor v2
```
🚀 Servidor de Setor Iniciado: [ORMUZ/SETOR_06]
🕒 Relógio Lógico Lamport inicializado em: 0
📥 Buffer de fila: 100 alertas | Starvation threshold: 3 ciclos críticos
==================================================
🤝 [ORMUZ/SETOR_06] Vizinho registado na malha: SETOR_07
📊 [QUEUE STATUS] Críticos: 0 | Normais: 0
```

### Quando alerta crítico chega
```
🚨 ALERTA CRÍTICO DETETADO [ORMUZ/RADAR_TCP]: NAVIO_APROXIMANDO em 40.2,-72.5
📥 Alerta CRÍTICO enfileirado para: 40.2,-72.5 | Fila crítica: 1
✅ Processando alerta CRÍTICO: 40.2,-72.5
⚖️ A iniciar Ricart-Agrawala -> Prioridade: 2 | Relógio: 42 | Destino: 40.2,-72.5
🏆 CONSENSO ALCANÇADO! Setor SETOR_06 ganhou a Exclusão Mútua.
🎯 Decisão P2P: O Drone escolhido foi o [SETOR_06/DRONE_1] (pertence ao setor SETOR_06)
🚀 Ordem de despacho enviada DIRETAMENTE ao drone local SETOR_06/DRONE_1!
🔓 A libertar a exclusão mútua. A enviar ACK para 0 vizinhos na fila de espera...
```

### Starvation Prevention Triggered
```
📥 Alerta NORMAL enfileirado para: 40.3,-72.6 | Fila normal: 1
[... 3 alertas críticos processados ...]
🚀 Starvation Prevention: alerta normal foi PROMOVIDO para CRÍTICO!
✅ Processando alerta CRÍTICO: 40.3,-72.6
```

### TLM Threshold
```
📤 Enviando JSON -> {"tipo":"TLM","remetente":"SENSOR_VENTO_01","valor":"71.45"}
🚨 [THRESHOLD ALERT] Sensor SENSOR_VENTO_01 detectou condição crítica: 71.45 > 70.00 (em 2 leituras)
```

---

## ✅ CHECKLIST PRÉ-DEPLOY

- [ ] Compilação sem warnings: `go build -v`
- [ ] Testes unitários passam: `go test ./...`
- [ ] Integração E2E: 4 setores + 20 drones + dashboard
- [ ] Stress test: 100 alertas/s sem panic
- [ ] Memory profile: heap < 50MB crescimento estável
- [ ] Dashboard mostra frota corretamente
- [ ] Cascade shutdown: todos drones marcam DESCONECTADO em <10s
- [ ] Starvation prevention ativado: log mostra 🚀 em ~30s com carga mista
- [ ] TLM threshold: log 🚨 quando valor > 70 por 2 leituras

---

## 📞 TROUBLESHOOTING

| Sintoma | Causa | Solução |
|---------|-------|---------|
| `undefined: IniciarRequisicaoDrone` | Função com CamelCase mas chamada com camelCase | Verificar consistência capitalização |
| `Queue STATUS: 0 ❌ | Críticos: 0` | Consumer não iniciou (skip `StartConsumer()`) | Valida `go AlertQueue.StartConsumer(gs)` chamado |
| Deadlock em `DequeueAlert()` | Nenhum alert enqu do, consumer bloqueado em `notEmpty.Wait()` | Simular enqueue com teste, ou timeout no wait |
| Memory leak | Conexões não fechadas | Validar `defer conn.Close()` em todos listeners |
| Race condition em FrotaGlobal | Acesso sem mutex | Garantir `FrotaMu.Lock/Unlock` em leituras/escritas |

---

**Data:** 2026-04-24  
**Versão:** v2.0.0-testing  
**Tipo:** QA Guide (Multi-Machine Scenarios)
