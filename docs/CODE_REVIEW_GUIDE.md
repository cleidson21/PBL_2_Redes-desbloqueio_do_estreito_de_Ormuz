# 🔍 GUIA DE REVISÃO - Servidor Modular v2.0

## Estrutura de Revisão

Este documento organiza os arquivos por ordem de revisão recomendada, com pontos-chave de atenção.

---

## 📖 ORDEM DE REVISÃO RECOMENDADA

### **1. types.go** (LEIA PRIMEIRO)
**Propósito:** Entender o modelo de dados  
**Tamanho:** 99 linhas

**Pontos-chave:**
- `Mensagem` struct - protocolo de rede
- `EstadoDrone` struct - estado da frota
- `Alert` struct - fila interna
- `AlertQueue` struct - **NEW** - sistema de priorização
- `GlobalState` struct - **NEW** - encapsulamento de estado global

**Checklist de revisão:**
- [ ] AlertQueue tem critical + normal queues
- [ ] AlertQueue.notEmpty é sync.Cond para bloqueio inteligente
- [ ] GlobalState agrupa 30+ variáveis antes globais
- [ ] NewGlobalState factory function inicializa corretamente

**Potenciais issues:**
- ❓ AlertQueue.mu protege ambas queues? Sim ✅
- ❓ Pode haver deadlock em notEmpty.Wait()? Não, signal() é chamado ✅

---

### **2. lamport.go** (LEIA SEGUNDO)  
**Propósito:** Validar relógio lógico  
**Tamanho:** 16 linhas

**Pontos-chave:**
- `TickLamport()` - incrementa + retorna
- `SyncLamport()` - sincroniza de valor remoto

**Checklist: **
- [ ] TickLamport() sempre incrementa antes de usar? Sim ✅
- [ ] SyncLamport() respeita max(local, remoto) + 1? Sim ✅

---

### **3. ricart.go** (LEIA TERCEIRO)
**Propósito:** Validar consenso distribuído  
**Tamanho:** 182 linhas

**Funções principais:**
- `IniciarRequisicaoDrone()` - Ricart requester
- `AvaliarPedidoVizinho()` - Ricart evaluator (delayed queue ou ACK)
- `ReceberAckP2P()` - Contador de ACKs
- `VerificarConsenso()` - Validação de consenso alcançado
- `ExecutarDespacho()` - Seleção de drone livre + envio de CMD
- `LiberarDrone()` - Liberação e processamento de fila de espera

**Checklist de revisão:**
- [ ] IniciarRequisicaoDrone() valida `estadoRicart == "LIVRE"`? Sim ✅
- [ ] Aging counter elevaa prioridade após 3 perdas? Sim, linha ~95 ✅
- [ ] AvaliarPedidoVizinho() implementa precedência corretamente? Sim ✅
  - Prioridade mais alta ganha
  - Mesma prioridade: menor tempo Lamport ganha
  - Mesma prioridade+tempo: ID lexical ganha (meuSetor < remetente)
- [ ] ExecutarDespacho() abandona graciosamente se nenhum drone LIVRE? Sim ✅
- [ ] LiberarDrone() processa fila de espera? Sim ✅

**Potenciais issues:**
- ❓ Pode haver race condition entre AvaliarPedido e ContadorAging? NÃO (muRicart.Lock) ✅
- ❓ ExecutarDespacho() pode deixar drone orphaned? Não, update é atômico ✅

---

### **4. queue.go** (LEIA QUARTO)
**Propósito:** Validar sistema de priorização + starvation  
**Tamanho:** 113 linhas

**Funções principais:**
- `EnqueueAlert()` - Adiciona alerta crítico ou normal
- `DequeueAlert()` - Remove alerta respeitando prioridade + starvation
- `QueueStats()` - Retorna counts (debug)
- `StartConsumer()` - Inicia goroutine dedicada

**Checklist:**
- [ ] EnqueueAlert() diferencia prioridades 1 vs 2? Sim ✅
- [ ] Fila normal tem limite de 100? Sim, checks `len(aq.normal) >= aq.maxSize` ✅
- [ ] Fila crítica cresce sem limite? Sim (design: críticos nunca descartados) ✅
- [ ] DequeueAlert() oferece prioridade a críticos? Sim ✅
- [ ] Starvation prevention acontece após N ciclos críticos? Sim, se `processedCount >= starveThreshold` ✅
- [ ] StartConsumer() chama IniciarRequisicaoDrone()? Sim ✅

**Potenciais issues:**
- ❓ DequeueAlert() bloqueia se vazio? Sim (by design, wait em cond) ✅
- ❓ Pode haver spurious wakeup de notEmpty.Wait()? Sim, mas loop `for` reconsidera ✅

---

### **5. p2p.go** (LEIA QUINTO)
**Propósito:** Validar malha P2P e gossip  
**Tamanho:** 110 linhas

**Funções principais:**
- `ListenP2P()` - Accept P2P connections
- `ConectarAosVizinhos()` - Dial to PEERS (reconnect loop)
- `ManipularMensagemP2P()` - Process P2P message (HELLO, GOSSIP, REQ, ACK, CMD)
- `RotinaGossip()` - Broadcast fleet state a cada 5s

**Checklist:**
- [ ] ListenP2P() corre em goroutine? Sim (main.go: `go ListenP2P()`) ✅
- [ ] ConectarAosVizinhos() reconecta em caso de falha? Sim, loop infinito com 5s delay ✅
- [ ] ManipularMensagemP2P() trata HELLO (registo)? Sim ✅
- [ ] P2P_REQ é delegado a AvaliarPedidoVizinho()? Sim ✅
- [ ] ACK incrementa counter? Sim ✅
- [ ] RotinaGossip() sincroniza frotaGlobal a cada 5s? Sim ✅
- [ ] Vizinho morto é detectado? Sim, fechamento de scanner/conn ✅

**Potenciais issues:**
- ❓ Pode haver deadlock em VizinhosMu? Não, lock patterns são curtos ✅
- ❓ Gossip floods a rede? 5s interval é razoável para 4 setores ✅

---

### **6. listeners.go** (LEIA SEXTO)
**Propósito:** Validar periféricos (sensores, drones, dashboard)  
**Tamanho:** 166 linhas

**Funções principais:**
- `HabilitarKeepAlive()` - TCP keep-alive
- `EnriquecerIdentidade()` - Adiciona namespace "ORMUZ/SETOR_XX"
- `AtualizarDashboards()` - Broadcast a todos dashboards
 - `ListenSensoresTLM()` - UDP listener (porta 48080)
 - `ListenRadarTCP()` - TCP listener (porta 48081) - EVT/ALERTA → EnqueueAlert(p=2)
 - `ListenDrones()` - TCP listener (porta 48082) - REG de drones + ACK de status
 - `ListenDashboardTCP()` - TCP listener (porta 48083) - CMD/REQUISICAO_MANUAL → EnqueueAlert(p=1)

**Checklist:**
- [ ] TLM vem via UDP (48080) e vai direto ao dashboard? Sim ✅
- [ ] Radar EVT/ALERTA entra na fila crítica? Sim, `EnqueueAlert(msg.Posicao, 2)` ✅
- [ ] Dashboard manual requisições entram na fila normal? Sim, `EnqueueAlert(msg.Posicao, 1)` ✅
- [ ] Drone REG registra conexão em `DronesLocais`? Sim ✅
- [ ] Drone desconexão marca DESCONECTADO na frota? Sim ✅
- [ ] Dashboard broadcast é feito para cada evento? Sim ✅

**Potenciais issues:**
- ❓ UDP pode perder TLM? SIM (UDP unreliable), esperado ✅
- ❓ TCP keep-alive previne hanging connections? Sim, 3s keep-alive ✅

---

### **7. main.go** (LEIA SÉTIMO)
**Propósito:** Validar orquestração final  
**Tamanho:** 47 linhas

**Pontos-chave:**
- Cria GlobalState com capacidade 100 + starvation threshold 3
- Inicia todos os listeners em goroutines
- Inicia P2P connections
- Inicia gossip routine
- **Inicia consumer da AlertQueue** ← CRÍTICO

**Checklist:**
- [ ] NewGlobalState() chamado com parâmetros corretos? Sim ✅
- [ ] AlertQueue.StartConsumer() é chamado? Sim, linha ~31 ✅
- [ ] select{} bloqueia main indefinidamente? Sim ✅
- [ ] Log de inicialização mostra buffer + threshold? Sim ✅

**Potenciais issues:**
- ❓ Consumer inicia antes de ports estarem listening? Não importa, consumer bloqueia até alert ✅

---

### **8. util.go** (LEIA POR ÚLTIMO)
**Propósito:** Validar helpers  
**Tamanho:** 10 linhas

**Funções:**
- `parseAddressList()` - Converte "addr1,addr2,..." em []string

**Checklist:**
- [ ] Respeitaespaços em branco? Sim, `TrimSpace()` ✅

---

## 🔗 FLUXO INTEGRADO (Visão Completa)

```
INICIALIZAÇÃO
├─ main.go cria GlobalState
├─ main.go inicia 5 listeners (P2P, TLM UDP, Radar TCP, Drones TCP, Dashboard TCP)
├─ main.go inicia RotinaGossip()
└─ main.go inicia AlertQueue.StartConsumer()

RECEBIMENTO DE ALERTA (Radar)
├─ Radar conecta em ListenRadarTCP (:48081)
├─ Recebe EVT/ALERTA
├─ EnriquecerIdentidade() adiciona namespace
├─ AtualizarDashboards() notifica dashboard
└─ EnqueueAlert(coordenada, prioridade=2) ← ENTRA NA FILA CRÍTICA

PROCESSAMENTO (Consumer Thread)
├─ DequeueAlert() retira alerta crítico (bloqueado até disponível)
├─ IniciarRequisicaoDrone(2, coordenada) ← RICART INICIA
│  ├─ MeuTempoPedido = TickLamport()
│  ├─ Envia P2P_REQ aos vizinhos
│  └─ Estado muda LIVRE → ESPERANDO
├─ Vizinhos recebem P2P_REQ em ManipularMensagemP2P()
│  └─ AvaliarPedidoVizinho() → ACK ou enfileira
├─ ReceberAckP2P() incrementa contador
├─ VerificarConsenso() quando acksRecebidos ≥ vizinhos
│  └─ ExecutarDespacho(coordenada)
│     ├─ Escolhe drone LIVRE
│     └─ Envia CMD ao drone local OU P2P_CMD ao vizinho
└─ LiberarDrone() → Estado LIVRE, processa fila de espera P2P

PROPAGAÇÃO DE ESTADO
├─ RotinaGossip() a cada 5s envia frota para vizinhos + dashboards
└─ Drones atualizam ACK com status (EM_MISSAO, LIVRE, DESCONECTADO)
```

---

## 🧪 DURANTE STRESS TESTING (Validação Prática)

### 2-Server Scenario (stress_sensores.sh 1 + stress_atuadores.sh 1)

**types.go + queue.go - Validar Fila:**
```bash
# Procurar por QUEUE STATUS durante processamento
docker logs servidor5 | grep "QUEUE STATUS"
# Esperado: "critical: 1-5 itens, normal: 0 itens" (consumo rápido)
# RED FLAG: "critical: 100+" ou "normal: 100" (fila presa)
```

**lamport.go - Validar Clock:**
```bash
# Validar incremento
docker logs servidor5 | grep "LAMPORT_CLOCK" | tail -10 | awk '{print $NF}'
# Esperado: valores crescentes (42 → 43 → 44...)
# RED FLAG: stagnação (sempre 42) ou saltos > 10
```

**ricart.go - Validar Consenso:**
```bash
# Procurar requisição → ACKs → consenso
docker logs servidor5 | grep -E "RICART_REQUEST|ACK_RECEIVED|CONSENSO" | head -20
# Esperado: 1 REQUEST → 1 ACK (de servidor6) → 1 CONSENSO
# RED FLAG: REQUEST mas nunca ACK (servidor6 offline?)
```

**p2p.go - Validar P2P:**
```bash
# Verificar conexão bidirecional
docker logs servidor5 | grep "Conectado\|VIZINHOS" | head -5
# Esperado: "Conectado a 172.16.201.6" ou lista de vizinhos
# RED FLAG: "Falha" ou "TIMEOUT" (problema de rede)
```

**listeners.go - Validar Registros:**
```bash
# Contar clientes registrados
docker logs servidor5 | grep "registrado em SETOR_05" | wc -l
# Esperado: >= 2 (1 sensor + 1 drone)
# RED FLAG: 0 (ninguém conectando) ou muitos timeouts
```

---

### 4-Server Scenario (stress_sensores.sh 3 + stress_atuadores.sh 3)

**Adicional: Gossip Replication (p2p.go)**
```bash
# Coletar FrotaGlobal de todos servidores
for i in 5 6 7 8; do
  echo "=== Servidor $i ===";
  docker logs servidor$i | grep "FrotaGlobal" | tail -1
done
# Esperado: mesma estrutura em todos (possível 1-2s de lag)
# RED FLAG: Servidor7 divergido vs Servidor5 (gossip falhou)
```

**Queue Breakdown (queue.go)**
```bash
# Ver distribuição de alertas nos 4 setores
for i in 5 6 7 8; do
  echo -n "Setor.$i: "
  docker logs servidor$i | grep "registrado em SETOR" | grep "sensor\|drone" | wc -l
done
# Esperado: distribuição ~similar (3-4 por setor)
# RED FLAG: 1 setor com 10, outro com 0 (load balance broken)
```

**Multi-server Ricart (ricart.go)**
```bash
# Validar consenso com >1 vizinho (quorum)
docker logs servidor5 | grep "ACK_RECEIVED" | head -20
# Esperado: cada consenso tem 3 ACKs (de .6, .7, .8)
# RED FLAG: consenso com <3 ACKs (quorum broken)
```

---

### Stress Scenario (stress_sensores.sh 20 + stress_atuadores.sh 20)

**Starvation Prevention (queue.go)**
```bash
# Procurar promoção de alertas normais
docker logs servidor5 | grep "PROMOVIDO_NORMAL_A_CRITICO" | wc -l
# Esperado: 10-50 durante stress (algumas promoções)
# RED FLAG: 0 (starvation disabled?) ou 1000+ (muito agressivo)
```

**Performance - Memory (main.go + all)**
```bash
# Monitorar crescimento de memória
watch -n 5 'docker stats servidor5 --no-stream | tail -1 | awk "{print \$4}"'
# Esperado: <200MB ao longo de 5min (estável)
# RED FLAG: crescimento contínuo (memory leak) ou >300MB
```

**Performance - CPU (util.go + listeners.go)**
```bash
# Monitorar CPU durante stress
docker stats servidor5 --no-stream | tail -1 | awk '{print $3}'
# Esperado: <70% durante stress
# RED FLAG: 99%+ (deadlock?) ou crescimento lento > 90% (backpressure fail)
```

**Throughput (listeners.go + queue.go)**
```bash
# Contar despachos/min durante stress
docker logs servidor5 --since 60s | grep "DESPACHO_CONFIRMADO" | wc -l
# Esperado: 100+ despachos/min com 20 sensores
# RED FLAG: <50 (engarrafamento) ou 0 (sistema travado)
```

**Latência E2E (ricart.go + listeners.go)**
```bash
# Extrair timestamp entrada vs saída
docker logs servidor5 | grep "ENTRADA\|DESPACHO" | \
  paste - - | awk '{print $2-$1}' | sort -n | tail -1
# Esperado: <8000ms no p99
# RED FLAG: >15000ms (consenso muito lento) ou timeout
```

---

### Failover Scenario (Kill 1 server, validar reconexão)

**P2P Recovery (p2p.go)**
```bash
# T0: Anotar timestamp
T0=$(date +%s)
docker stop servidor5
echo "Killed at T=$T0"

# T0+5: Verificar reconexão de drones/sensores
docker logs drone_1 | grep "RECONECTANDO\|NOVO_GATEWAY" | head -1
# Esperado: encontrado com timestamp < T0+10s
# RED FLAG: não encontrado (failover quebrou)
```

**Gossip During Downtime (p2p.go)**
```bash
# Enquanto servidor5 está down, verificar sync nos outros
docker logs servidor6 | grep "SINCRONIZANDO\|LAMPORT_CLOCK" | tail -5
# Esperado: continua processando sem servidor5
# RED FLAG: "ESPERANDO SERVIDOR 05" (hard dependency)
```

**State Recover (ricart.go + queue.go)**
```bash
# Depois de restart servidor5
docker start servidor5
sleep 5
docker logs servidor5 | grep "SINCRONIZANDO\|LAMPORT.*CATCH_UP"
# Esperado: vê backlog de eventos missed
# RED FLAG: começa do 0 (perdeu histórico)
```

---

## 🚨 RED FLAGS (O Que Procurar)

| Issue | Onde Procurar | Gravidade |
|-------|---------------|----------|
| **Deadlock em mutex** | ricart.go (múltiplos locks), queue.go (notEmpty) | 🔴 CRÍTICA |
| **Race condition em FrotaGlobal** | listeners.go (REG + ACK sem lock) | 🔴 CRÍTICA |
| **Alertas perdidos** | queue.go (overflow de fila) | 🟠 ALTA |
| **Memory leak conexões** | listeners.go (sem defer Close) | 🟠 ALTA |
| **Buzzy waiting loop** | queue.go (notEmpty.Wait loop) | 🟡 MODERADA |
| **Consumer não inicia** | main.go (sem StartConsumer call) | 🟡 MODERADA |

---

## ✅ CHECKLIST FINAL DE REVIEW

### Estrutura
- [ ] Todos 8 ficheiros compilam sem warnings
- [ ] Nenhum import circular
- [ ] Exposição correta (Caso de Pascal = público)

### Funcionalidade
- [ ] AlertQueue bufferiza alertas corretamente
- [ ] Starvation prevention promo iona N→2 após 3 ciclos
- [ ] Consenso Ricart-Agrawala funciona distribuído
- [ ] TLM otimizado (2s intervalo)
- [ ] Dashboard recebe atualizações em tempo real

### Performance
- [ ] Nenhum goroutine leak
- [ ] Nenhuma contenção excessiva de mutex
- [ ] Memory footprint razoável (< 50MB)

### Compatibilidade
- [ ] Protocolos P2P inalterados
- [ ] Portas TCP/UDP inalteradas
- [ ] Docker build compatível

### Documentação
- [ ] Código comentado nos pontos complexos
- [ ] Funções exportadas têm docstring
- [ ] CHANGELOG documenta todas mudanças

---

## 📞 PERGUNTAS COMUNS DURANTE REVIEW

**P: Por que AlertQueue tem ambas critical + normal?**
A: Para garantir que alertas normais (low priority) não morrem de fome enquanto críticos (high priority) são processados.

**P: Por que starvation threshold = 3?**
A: Tuning empírico - permite 3 ciclos críticos antes de promoção, balanceando responsividade vs normalidade.

**P: Por que TLM intervalo 2s (antes 500ms)?**
A: 500ms = 120 msgs/min = saturação UDP. 2s = 30 msgs/min reduz carga 75% mantendo detecção adequada.

**P: Pode haver deadlock?**
A: Improvável - locks são curtos e ordenados (muRicart depois muVizinhos). notEmpty.Wait() aguarda signal() correspondente.

**P: Por que GlobalState struct em vez de variáveis globais?**
A: Suporta múltiplas instâncias (testes paralelos), facilita dependency injection, reduz acoplamento.

---

**Data:** 2026-04-24  
**Tipo:** Code Review Guide  
**Versão:** 2.0.0
