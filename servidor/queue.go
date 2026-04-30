package main

import (
	"fmt"
	"time"
)

// EnqueueAlert adiciona um alerta à fila apropriada (crítico ou normal)
// Retorna true se enfileirado com sucesso, false se fila cheia
func (aq *AlertQueue) EnqueueAlert(coordenada string, prioridade int) bool {
	aq.mu.Lock()
	defer aq.mu.Unlock()

	alert := Alert{
		Coordenada:    coordenada,
		Prioridade:    prioridade,
		Timestamp:     time.Now().UnixNano(),
		ID:            fmt.Sprintf("%d", time.Now().UnixNano()),
		StarveCounter: 0,
	}

	if prioridade == 2 {
		// Fila crítica: verifica se não extrapolou tamanho máximo
		if len(aq.critical) >= aq.maxSize {
			fmt.Printf("⚠️ Fila crítica CHEIO! Descartando alerta mais antigo.\n")
			if len(aq.critical) > 0 {
				aq.critical = aq.critical[1:]
			}
		}
		aq.critical = append(aq.critical, alert)
		fmt.Printf("📥 Alerta CRÍTICO enfileirado para: %s | Fila crítica: %d\n", coordenada, len(aq.critical))
	} else {
		// Fila normal: verifica se não extrapolou tamanho máximo
		if len(aq.normal) >= aq.maxSize {
			fmt.Printf("⚠️ Fila normal CHEIA! Descartando alerta mais antigo.\n")
			if len(aq.normal) > 0 {
				aq.normal = aq.normal[1:]
			}
		}
		aq.normal = append(aq.normal, alert)
		fmt.Printf("📥 Alerta NORMAL enfileirado para: %s | Fila normal: %d\n", coordenada, len(aq.normal))
	}

	// Notifica consumidor
	aq.notEmpty.Signal()
	return true
}

// DequeueAlert remove e retorna o próximo alerta respeitando prioridade e starvation prevention
// Bloqueia até que um alerta esteja disponível
func (aq *AlertQueue) DequeueAlert() Alert {
	aq.mu.Lock()
	defer aq.mu.Unlock() // Garante que a fila será SEMPRE destravada no final!

	// Enquanto ambas as filas estiverem vazias, durma
	for len(aq.critical) == 0 && len(aq.normal) == 0 {
		aq.notEmpty.Wait() // Destrava aq.mu, dorme. Quando acorda, TRAVA aq.mu novamente e reavalia o "for"
	}

	// Regra 1: Starvation Prevention (Normal -> Crítico)
	if len(aq.normal) > 0 && aq.processedCount >= aq.starveThreshold {
		alert := aq.normal[0]
		aq.normal = aq.normal[1:]
		alert.Prioridade = 2  // Promoção
		aq.processedCount = 0 // Reset
		fmt.Printf("🚀 Starvation Prevention: alerta normal foi PROMOVIDO para CRÍTICO!\n")
		return alert
	}

	// Regra 2: Alertas Críticos têm preferência
	if len(aq.critical) > 0 {
		alert := aq.critical[0]
		aq.critical = aq.critical[1:]
		aq.processedCount++
		fmt.Printf("✅ Processando alerta CRÍTICO: %s\n", alert.Coordenada)
		return alert
	}

	// Regra 3: Alertas Normais (Sobrou apenas este)
	alert := aq.normal[0]
	aq.normal = aq.normal[1:]
	fmt.Printf("✅ Processando alerta NORMAL: %s\n", alert.Coordenada)
	return alert
}

// QueueStats retorna estatísticas da fila
func (aq *AlertQueue) QueueStats() (criticalCount, normalCount int) {
	aq.mu.Lock()
	defer aq.mu.Unlock()
	return len(aq.critical), len(aq.normal)
}

// StartConsumer inicia a goroutine consumidora que processa alertas da fila
func (aq *AlertQueue) StartConsumer(gs *GlobalState) {
	go func() {
		fmt.Println("✅ Consumer da fila INICIADO e aguardando alertas...")
		for {
			// O Consumidor pega o alerta da fila
			alert := aq.DequeueAlert()
			fmt.Printf("🎯 Consumer processando alerta: prioridade=%d, coordenada=%s\n", alert.Prioridade, alert.Coordenada)

			// E chama o Ricart. A própria função do Ricart já sabe esperar educadamente se estiver ocupada!
			IniciarRequisicaoDrone(gs, alert.Prioridade, alert.Coordenada)
		}
	}()
}
