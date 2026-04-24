package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	// Lê variáveis de ambiente
	meuSetor := os.Getenv("MEU_SETOR")
	if meuSetor == "" {
		meuSetor = "DESCONHECIDO"
	}

	peersEnv := os.Getenv("PEERS")

	// Cria estado global com buffer de 100 alertas e threshold de starvation prevention = 3
	gs := NewGlobalState(meuSetor, 100, 3)

	fmt.Printf("🚀 Servidor de Setor Iniciado: [%s]\n", gs.MeuNamespace)
	fmt.Printf("🕒 Relógio Lógico Lamport inicializado em: %d\n", gs.Relogio)
	fmt.Printf("📥 Buffer de fila: 100 alertas | Starvation threshold: 3 ciclos críticos\n")
	fmt.Println("==================================================")

	// Inicia listeners para diferentes portas
	go ListenP2P(gs)
	go ListenSensoresTLM(gs)
	go ListenRadarTCP(gs)
	go ListenDrones(gs)
	go ListenDashboardTCP(gs)

	time.Sleep(3 * time.Second)

	// Inicia conexões aos vizinhos P2P
	go ConectarAosVizinhos(gs, peersEnv)

	// Inicia rotina de gossip
	go RotinaGossip(gs)

	// Inicia consumer da fila de alertas
	gs.AlertQueue.StartConsumer(gs)

	// Log periódico do estado da fila (para debug)
	go func() {
		for {
			time.Sleep(10 * time.Second)
			critCount, normCount := gs.AlertQueue.QueueStats()
			fmt.Printf("📊 [QUEUE STATUS] Críticos: %d | Normais: %d\n", critCount, normCount)
		}
	}()

	// Bloqueia indefinidamente
	select {}
}
