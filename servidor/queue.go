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
	for {
		aq.mu.Lock()

		// Regra de starvation prevention: se N alertas críticos foram processados,
		// promoção automática de um alerta normal para crítico
		if len(aq.normal) > 0 && aq.processedCount >= aq.starveThreshold {
			alert := aq.normal[0]
			aq.normal = aq.normal[1:]
			alert.Prioridade = 2  // promoção
			aq.processedCount = 0 // reset contador
			aq.mu.Unlock()

			fmt.Printf("🚀 Starvation Prevention: alerta normal foi PROMOVIDO para CRÍTICO!\n")
			return alert
		}

		// Se há alertas críticos, sempre processa primeiro
		if len(aq.critical) > 0 {
			alert := aq.critical[0]
			aq.critical = aq.critical[1:]
			aq.processedCount++ // incrementa contador de ciclos críticos
			aq.mu.Unlock()

			fmt.Printf("✅ Processando alerta CRÍTICO: %s\n", alert.Coordenada)
			return alert
		}

		// Caso contrário, processa alerta normal se existir
		if len(aq.normal) > 0 {
			alert := aq.normal[0]
			aq.normal = aq.normal[1:]

			// Incrementa starvation counter se há críticos esperando
			if len(aq.critical) > 0 {
				alert.StarveCounter++
			}

			aq.mu.Unlock()

			fmt.Printf("✅ Processando alerta NORMAL: %s\n", alert.Coordenada)
			return alert
		}

		// Nenhum alerta disponível; aguarda sinal
		aq.notEmpty.Wait()
	}
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
		for {
			alert := aq.DequeueAlert()
			fmt.Printf("🎯 Consumer processando alerta: prioridade=%d, coordenada=%s\n", alert.Prioridade, alert.Coordenada)

			// Inicia requisição de drone com a prioridade do alerta
			IniciarRequisicaoDrone(gs, alert.Prioridade, alert.Coordenada)
			// Pequeno delay para evitar processamento muito rápido
			time.Sleep(100 * time.Millisecond)
		}
	}()
}
