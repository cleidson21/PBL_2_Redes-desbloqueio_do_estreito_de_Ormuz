package main

import (
	"fmt"
	"time"
)

// EnqueueAlert insere um alerta na fila correta respeitando o limite de capacidade de cada prioridade.
func (aq *AlertQueue) EnqueueAlert(gs *GlobalState, coordenada string, prioridade int, idRequisicao string) bool {
	aq.mu.Lock()
	defer aq.mu.Unlock()

	alert := Alert{
		Coordenada:    coordenada,
		Prioridade:    prioridade,
		Timestamp:     time.Now().UnixNano(),
		ID:            idRequisicao,
		StarveCounter: 0,
	}
	if alert.ID == "" {
		alert.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	if prioridade == 2 {
		if len(aq.critical) >= aq.maxSize {
			fmt.Printf("⚠️ Fila crítica CHEIA! Alerta crítico rejeitado para: %s\n", coordenada)
			return false
		}
		aq.critical = append(aq.critical, alert)
		fmt.Printf("📥 Alerta CRÍTICO enfileirado para: %s | Fila crítica: %d\n", coordenada, len(aq.critical))
	} else {
		if len(aq.normal) >= aq.maxSize {
			fmt.Printf("⚠️ Fila normal CHEIA! Alerta normal rejeitado para: %s\n", coordenada)
			return false
		}
		aq.normal = append(aq.normal, alert)
		fmt.Printf("📥 Alerta NORMAL enfileirado para: %s | Fila normal: %d\n", coordenada, len(aq.normal))
	}

	alert.Lamport = TickLamport(gs)
	EnviarEventoRequisicao(gs, alert.ID, "ENQUEUED", alert.Prioridade, alert.Lamport)
	aq.notEmpty.Signal()
	return true
}

// DequeueAlert bloqueia até existir trabalho e aplica a política de starvation prevention.
func (aq *AlertQueue) DequeueAlert() Alert {
	aq.mu.Lock()
	defer aq.mu.Unlock()

	for len(aq.critical) == 0 && len(aq.normal) == 0 {
		aq.notEmpty.Wait()
	}

	if len(aq.normal) > 0 && aq.processedCount >= aq.starveThreshold {
		alert := aq.normal[0]
		aq.normal = aq.normal[1:]
		alert.Prioridade = 2
		aq.processedCount = 0
		fmt.Printf("🚀 Starvation Prevention: alerta normal foi PROMOVIDO para CRÍTICO!\n")
		return alert
	}

	if len(aq.critical) > 0 {
		alert := aq.critical[0]
		aq.critical = aq.critical[1:]
		aq.processedCount++
		return alert
	}

	alert := aq.normal[0]
	aq.normal = aq.normal[1:]
	return alert
}

func (aq *AlertQueue) QueueStats() (criticalCount, normalCount int) {
	aq.mu.Lock()
	defer aq.mu.Unlock()
	return len(aq.critical), len(aq.normal)
}

// StartConsumer executa o consumo de alertas quando há recurso de despacho disponível.
func (aq *AlertQueue) StartConsumer(gs *GlobalState) {
	go func() {
		for {
			// O consumidor só avança quando não há exclusão mútua ativa e existe ao menos um drone livre.
			for {
				gs.RicartMu.Lock()
				isLivre := (gs.EstadoRicart == "LIVRE")
				gs.RicartMu.Unlock()

				gs.FrotaMu.RLock()
				dronesLivres := 0
				for _, drone := range gs.FrotaGlobal {
					if drone.Status == "LIVRE" {
						dronesLivres++
					}
				}
				gs.FrotaMu.RUnlock()

				if isLivre && dronesLivres > 0 {
					break
				}

				time.Sleep(100 * time.Millisecond)
			}

			alert := aq.DequeueAlert()

			IniciarRequisicaoDrone(gs, alert.ID, alert.Prioridade, alert.Coordenada)
		}
	}()
}
