# GUIA DE TESTES - Sistema de Filas Modularizado

## 🧪 TESTES RÁPIDOS LOCAL

### 1. **Compilação Modular** ✅
```bash
cd servidor
go build -o servidor_local

cd ../sensor_tlm
go build -o sensor_tlm_local
```
**Esperado:** Sem erros, executáveis em ~2s

---

## 📐 TESTES DE UNIDADE PROPOSTOS

### A. **queue.go - ProducerConsumer**

```go
// test_queue.go
func TestEnqueueDequeueOrder(t *testing.T) {
    aq := &AlertQueue{
        critical: make([]Alert, 0, 100),
        normal: make([]Alert, 0, 100),
        maxSize: 100,
        starveThreshold: 3,
    }
    aq.notEmpty = sync.NewCond(&aq.mu)
    
    // Enqueue crítico é prioritário
    aq.EnqueueAlert("A", 2)
    aq.EnqueueAlert("B", 1)
    aq.EnqueueAlert("C", 2)
    
    // Deve processar: C, A, B (críticos primeiro)
    a1 := aq.DequeueAlert()
    if a1.Coordenada != "C" { t.Fail() }
    
    a2 := aq.DequeueAlert()
    if a2.Coordenada != "A" { t.Fail() }
}

func TestStarvationPrevention(t *testing.T) {
    aq := &AlertQueue{...}
    
    // Enqueue: 4 críticos + 1 normal
    for i := 0; i < 4; i++ {
        aq.EnqueueAlert(fmt.Sprintf("CRIT_%d", i), 2)
    }
    aq.EnqueueAlert("NORMAL_1", 1)
    
    // Consume 3 críticos: processedCount → 3
    for i := 0; i < 3; i++ {
        _ = aq.DequeueAlert() // consume CRIT_*
    }
    
    // Próximo: normal_1 PROMOVIDO a 2 após 3 ciclos
    alertPromocionado := aq.DequeueAlert()
    if alertPromocionado.Prioridade != 2 {
        t.Errorf("Starvation prevention falhou: prioridade=%d, esperava 2", alertPromocionado.Prioridade)
    }
}
```

---

### B. **ricart.go - Consenso**

```go
// test_ricart_test.go
func TestRicartInitiation(t *testing.T) {
    gs := NewGlobalState("SETOR_06", 100, 3)
    gs.Vizinhos["SETOR_07"] = mockConn1
    gs.Vizinhos["SETOR_08"] = mockConn2
    
    IniciarRequisicaoDrone(gs, 1, "40.2,-72.5")
    
    // Deve estar ESPERANDO
    if gs.EstadoRicart != "ESPERANDO" {
        t.Fail()
    }
    
    // Lamport deve ter incrementado
    if gs.MeuTempoPedido == 0 {
        t.Fail()
    }
}

func TestExecutarDespacho_NoDronesFree(t *testing.T) {
    gs := NewGlobalState("SETOR_06", 100, 3)
    gs.FrotaGlobal["DRONE_1"] = EstadoDrone{"EM_MISSAO", "SETOR_06"}
    
    // Nenhum LIVRE → deve fazer LiberarDrone()
    ExecutarDespacho(gs, "40.2,-72.5")
    
    if gs.EstadoRicart != "LIVRE" {
        t.Fail()
    }
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
export MEU_SETOR=SETOR_06 PEERS=localhost:8084
./servidor_local

# Terminal 2
export SENSOR_ID=SENSOR_VENTO_01 SERVER_ADDRS=localhost:8080
./sensor_tlm_local

# Monitorar output: deve ver "🚨 [THRESHOLD ALERT]" quando valor > 70 por 2 leituras
```

**Esperado:** A cada ~4s (2x intervalo de 2s), sensor reporta quando valor > 70

---

### Cenário 3: Gossip propagação com nova queue

**Setup:**
```bash
# Simular 2 setor (em máquinas diferentes ou portas diferentes)
SETOR_06: :8084 (P2P), :8080 (UDP), :8081-83 (TCP)
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
    nc -w 1 localhost 8081 &
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

**Data:** 2025-01-02  
**Versão:** v2.0.0-testing  
**Tipo:** QA Guide
