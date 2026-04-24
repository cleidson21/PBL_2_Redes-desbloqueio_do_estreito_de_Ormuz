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

**Checklist:**
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
- `ListenSensoresTLM()` - UDP listener (porta 8080)
- `ListenRadarTCP()` - TCP listener (porta 8081) - EVT/ALERTA → EnqueueAlert(p=2)
- `ListenDrones()` - TCP listener (porta 8082) - REG de drones + ACK de status
- `ListenDashboardTCP()` - TCP listener (porta 8083) - CMD/REQUISICAO_MANUAL → EnqueueAlert(p=1)

**Checklist:**
- [ ] TLM vem via UDP (8080) e vai direto ao dashboard? Sim ✅
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
├─ Radar conecta em ListenRadarTCP (:8081)
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

**Data:** 2025-01-02  
**Tipo:** Code Review Guide  
**Versão:** 2.0.0
