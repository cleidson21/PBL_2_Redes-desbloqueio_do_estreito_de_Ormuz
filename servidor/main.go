package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	meuSetor := os.Getenv("MEU_SETOR")
	if meuSetor == "" {
		meuSetor = "DESCONHECIDO"
	}

	peersEnv := os.Getenv("PEERS")

	gs := NewGlobalState(meuSetor, 100, 3)

	fmt.Printf("🚀 Servidor de Setor Iniciado: [%s]\n", gs.MeuNamespace)
	fmt.Printf("🕒 Relógio Lógico Lamport inicializado em: %d\n", gs.Relogio)
	fmt.Printf("📥 Buffer de fila: 100 alertas | Starvation threshold: 3 ciclos críticos\n")
	fmt.Println("==================================================")

	go ListenP2P(gs)
	go ListenSensoresTLM(gs)
	go ListenRadarTCP(gs)
	go ListenDrones(gs)
	go ListenDashboardTCP(gs)

	time.Sleep(3 * time.Second)

	go ConectarAosVizinhos(gs, peersEnv)

	go RotinaGossip(gs)

	gs.AlertQueue.StartConsumer(gs)

	go func() {
		for {
			time.Sleep(10 * time.Second)
			LimparFrotaExpirada(gs, 45*time.Second)
			critCount, normCount := gs.AlertQueue.QueueStats()

			gs.FrotaMu.RLock()
			qtdDronesLivres := 0
			qtdDronesEmMissao := 0
			qtdDronesDesconectados := 0
			for _, estado := range gs.FrotaGlobal {
				switch estado.Status {
				case "LIVRE":
					qtdDronesLivres++
				case "EM_MISSAO":
					qtdDronesEmMissao++
				case "DESCONECTADO":
					qtdDronesDesconectados++
				}
			}
			gs.FrotaMu.RUnlock()

			fmt.Printf("📊 [QUEUE STATUS] Críticos: %d | Normais: %d | Drones: ✅%d | 🚁%d | ❌%d\n",
				critCount, normCount, qtdDronesLivres, qtdDronesEmMissao, qtdDronesDesconectados)
		}
	}()

	select {}
}
