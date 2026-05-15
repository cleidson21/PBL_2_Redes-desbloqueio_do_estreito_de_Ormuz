package main

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// EnviarEventoRequisicao replica o estado da requisição para os dashboards conectados.
func EnviarEventoRequisicao(gs *GlobalState, id string, status string, prioridade int, lamport int) {
	AtualizarDashboards(gs, Mensagem{
		Tipo:       "REQ_UPDATE",
		Remetente:  id,
		Acao:       status,
		Prioridade: prioridade,
		Relogio:    lamport,
	})
}

// IniciarRequisicaoDrone inicia a negociação de exclusão mútua antes de despachar um drone.
func IniciarRequisicaoDrone(gs *GlobalState, id string, prioridadeInicial int, coordenada string) {
	// A requisição só pode prosseguir quando a seção crítica local estiver realmente livre.
	for {
		gs.RicartMu.Lock()
		if gs.EstadoRicart == "LIVRE" {
			break
		}
		gs.RicartMu.Unlock()
		time.Sleep(50 * time.Millisecond)
	}
	if gs.ContadorAging >= 3 {
		fmt.Printf("🔥 [AGING] Setor %s cansou de perder a vez! Prioridade elevada de %d para 2 (CRÍTICA)\n", gs.MeuSetor, prioridadeInicial)
		prioridadeInicial = 2
	}

	gs.EstadoRicart = "ESPERANDO"
	gs.RequisicaoAtualID = id
	gs.MinhaPrioridade = prioridadeInicial
	gs.AlvoAtual = coordenada
	gs.AcksRecebidos = 0
	gs.MeuTempoPedido = TickLamport(gs)
	EnviarEventoRequisicao(gs, id, "WAITING", prioridadeInicial, gs.MeuTempoPedido)
	gs.RicartMu.Unlock()

	gs.VizinhosMu.RLock()
	qtdVizinhos := len(gs.Vizinhos)
	if qtdVizinhos == 0 {
		gs.VizinhosMu.RUnlock()
		VerificarConsenso(gs)
		return
	}

	msgReq := Mensagem{
		Tipo:       "P2P_REQ",
		Remetente:  gs.MeuSetor,
		Relogio:    gs.MeuTempoPedido,
		Prioridade: prioridadeInicial,
	}
	payload, _ := json.Marshal(msgReq)

	for _, conn := range gs.Vizinhos {
		fmt.Fprintf(conn, "%s\n", payload)
	}
	gs.VizinhosMu.RUnlock()

	// O timeout protege a malha contra espera infinita quando algum ACK não retorna.
	go MonitorConsensoComTimeout(gs, 15*time.Second)
}

// AvaliarPedidoVizinho decide entre adiar a resposta ou liberar o vizinho com ACK.
func AvaliarPedidoVizinho(gs *GlobalState, msgReq Mensagem, connVizinho net.Conn) {
	gs.RicartMu.Lock()
	defer gs.RicartMu.Unlock()

	devoAtrasar := false

	if gs.EstadoRicart == "USANDO" {
		devoAtrasar = true
	} else if gs.EstadoRicart == "ESPERANDO" {
		if gs.MinhaPrioridade > msgReq.Prioridade {
			devoAtrasar = true
		} else if gs.MinhaPrioridade == msgReq.Prioridade {
			if gs.MeuTempoPedido < msgReq.Relogio {
				devoAtrasar = true
			} else if gs.MeuTempoPedido == msgReq.Relogio {
				if gs.MeuSetor < msgReq.Remetente {
					devoAtrasar = true
				}
			}
		}
	}

	if devoAtrasar {
		gs.FilaDeEspera = append(gs.FilaDeEspera, msgReq)
	} else {
		if gs.EstadoRicart == "ESPERANDO" {
			gs.ContadorAging++
		}
		ackMsg := Mensagem{
			Tipo:      "ACK",
			Remetente: gs.MeuSetor,
			Destino:   msgReq.Remetente,
		}
		payload, _ := json.Marshal(ackMsg)
		fmt.Fprintf(connVizinho, "%s\n", payload)
	}
}

// ReceberAckP2P contabiliza ACKs recebidos e reavalia se o consenso já foi atingido.
func ReceberAckP2P(gs *GlobalState) {
	gs.RicartMu.Lock()
	if gs.EstadoRicart == "ESPERANDO" {
		gs.AcksRecebidos++
	}
	gs.RicartMu.Unlock()
	VerificarConsenso(gs)
}

// VerificarConsenso confirma a posse da seção crítica quando todos os vizinhos já responderam.
func VerificarConsenso(gs *GlobalState) {
	gs.RicartMu.Lock()
	defer gs.RicartMu.Unlock()

	if gs.EstadoRicart != "ESPERANDO" {
		return
	}

	gs.VizinhosMu.RLock()
	vivos := len(gs.Vizinhos)
	gs.VizinhosMu.RUnlock()

	if gs.AcksRecebidos >= vivos {
		gs.EstadoRicart = "USANDO"
		gs.ContadorAging = 0
		fmt.Printf("🏆 [RICART] CONSENSO ALCANÇADO! (Req: Prioridade=%d, Lamport=%d) | ACKs: %d/%d\n", gs.MinhaPrioridade, gs.MeuTempoPedido, gs.AcksRecebidos, vivos)

		// O despacho roda fora do lock para não bloquear novas mensagens de controle.
		go ExecutarDespacho(gs, gs.RequisicaoAtualID, gs.AlvoAtual)
	}
}

// ExecutarDespacho escolhe um drone livre e encaminha o comando para o setor correto.
func ExecutarDespacho(gs *GlobalState, requisicaoID string, coordenada string) {
	var droneEscolhido string
	var setorDoDrone string

	gs.FrotaMu.RLock()
	qtdDronesLivres := 0
	for id, estado := range gs.FrotaGlobal {
		if estado.Status == "LIVRE" {
			qtdDronesLivres++
			if droneEscolhido == "" {
				droneEscolhido = id
				setorDoDrone = estado.Setor
			}
		}
	}
	gs.FrotaMu.RUnlock()

	if droneEscolhido == "" {
		if ok := gs.AlertQueue.EnqueueAlert(gs, coordenada, gs.MinhaPrioridade, requisicaoID); !ok {
			fmt.Printf("⚠️ [RACE CONDITION] Falha ao re-enfileirar alerta %s após perder o drone.\n", coordenada)
		}
		fmt.Printf("⚠️ [RACE CONDITION] Consenso ganho, mas drones foram tomados por vizinhos. Abortando e re-enfileirando alerta (%s).\n", coordenada)
		LiberarDrone(gs)
		return
	}

	gs.FrotaMu.Lock()
	if estado, ok := gs.FrotaGlobal[droneEscolhido]; ok {
		estado.Status = "EM_MISSAO"
		estado.SeenAt = time.Now().UnixNano()
		gs.FrotaGlobal[droneEscolhido] = estado
	}
	gs.FrotaMu.Unlock()

	if setorDoDrone == gs.MeuSetor {
		gs.DronesMu.RLock()
		connDrone, ok := gs.DronesLocais[droneEscolhido]
		gs.DronesMu.RUnlock()

		if ok {
			cmdMsg := Mensagem{
				Tipo:    "CMD",
				Acao:    "DESPACHAR",
				Posicao: coordenada,
			}
			payload, _ := json.Marshal(cmdMsg)
			n, err := fmt.Fprintf(connDrone, "%s\n", payload)
			if err != nil {
				fmt.Printf("❌ Erro ao enviar comando ao drone local %s: %v (bytes: %d)\n", droneEscolhido, err, n)
				gs.FrotaMu.Lock()
				if estado, existe := gs.FrotaGlobal[droneEscolhido]; existe {
					estado.Status = "LIVRE"
					estado.SeenAt = time.Now().UnixNano()
					gs.FrotaGlobal[droneEscolhido] = estado
				}
				gs.FrotaMu.Unlock()
			} else {
				fmt.Printf("🎯 [DESPACHO LOCAL] Alvo: %s | Drone: %s (P:%d | L:%d)\n", coordenada, droneEscolhido, gs.MinhaPrioridade, gs.MeuTempoPedido)
				EnviarEventoRequisicao(gs, requisicaoID, "EXECUTED", gs.MinhaPrioridade, gs.MeuTempoPedido)
			}
		} else {
			fmt.Printf("⚠️ Drone local %s não está conectado em DronesLocais!\n", droneEscolhido)
			gs.FrotaMu.Lock()
			if estado, existe := gs.FrotaGlobal[droneEscolhido]; existe {
				estado.Status = "LIVRE"
				estado.SeenAt = time.Now().UnixNano()
				gs.FrotaGlobal[droneEscolhido] = estado
			}
			gs.FrotaMu.Unlock()
		}
	} else {
		gs.VizinhosMu.RLock()
		connVizinho, ok := gs.Vizinhos[setorDoDrone]
		gs.VizinhosMu.RUnlock()

		if ok {
			cmdMsg := Mensagem{
				Tipo:    "P2P_CMD",
				Destino: droneEscolhido,
				Acao:    "DESPACHAR",
				Posicao: coordenada,
			}
			payload, _ := json.Marshal(cmdMsg)
			n, err := fmt.Fprintf(connVizinho, "%s\n", payload)
			if err != nil {
				fmt.Printf("❌ Erro ao enviar P2P_CMD para setor %s: %v (bytes: %d)\n", setorDoDrone, err, n)
				gs.FrotaMu.Lock()
				if estado, existe := gs.FrotaGlobal[droneEscolhido]; existe {
					estado.Status = "LIVRE"
					estado.SeenAt = time.Now().UnixNano()
					gs.FrotaGlobal[droneEscolhido] = estado
				}
				gs.FrotaMu.Unlock()
			} else {
				fmt.Printf("🎯 [DESPACHO P2P] Alvo: %s | Drone: %s (Gateway: %s) (P:%d | L:%d)\n", coordenada, droneEscolhido, setorDoDrone, gs.MinhaPrioridade, gs.MeuTempoPedido)
				EnviarEventoRequisicao(gs, requisicaoID, "EXECUTED", gs.MinhaPrioridade, gs.MeuTempoPedido)
			}
		} else {
			fmt.Printf("⚠️ Vizinho %s não está conectado em Vizinhos!\n", setorDoDrone)
			gs.FrotaMu.Lock()
			if estado, existe := gs.FrotaGlobal[droneEscolhido]; existe {
				estado.Status = "LIVRE"
				estado.SeenAt = time.Now().UnixNano()
				gs.FrotaGlobal[droneEscolhido] = estado
			}
			gs.FrotaMu.Unlock()
		}
	}

	LiberarDrone(gs)
}

// LiberarDrone encerra a seção crítica e libera os vizinhos que ficaram em espera.
func LiberarDrone(gs *GlobalState) {
	gs.RicartMu.Lock()
	defer gs.RicartMu.Unlock()

	gs.EstadoRicart = "LIVRE"
	gs.RequisicaoAtualID = ""

	gs.VizinhosMu.RLock()
	for _, reqAntiga := range gs.FilaDeEspera {
		if conn, existe := gs.Vizinhos[reqAntiga.Remetente]; existe {
			ackMsg := Mensagem{
				Tipo:      "ACK",
				Remetente: gs.MeuSetor,
				Destino:   reqAntiga.Remetente,
			}
			payload, _ := json.Marshal(ackMsg)
			fmt.Fprintf(conn, "%s\n", payload)
		}
	}
	gs.VizinhosMu.RUnlock()

	gs.FilaDeEspera = nil
}

// MonitorConsensoComTimeout evita bloqueio permanente quando o consenso não se completa a tempo.
func MonitorConsensoComTimeout(gs *GlobalState, timeout time.Duration) {
	time.Sleep(timeout)

	gs.RicartMu.Lock()
	defer gs.RicartMu.Unlock()

	if gs.EstadoRicart == "ESPERANDO" {
		fmt.Printf("⏱️ TIMEOUT: Ricart-Agrawala timeout após %v. Resetando para LIVRE para tentar novamente.\n", timeout)
		gs.EstadoRicart = "LIVRE"
		gs.FilaDeEspera = nil
	}
}
