package main

import (
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// IniciarRequisicaoDrone inicia o protocolo Ricart-Agrawala para obter acesso exclusivo
func IniciarRequisicaoDrone(gs *GlobalState, prioridadeInicial int, coordenada string) {
	// 🔴 CORREÇÃO: Aguardar SINCRONAMENTE até Ricart estar LIVRE
	// Isso evita que alertas sejam descartados quando Ricart está ocupado
	for {
		gs.RicartMu.Lock()
		if gs.EstadoRicart == "LIVRE" {
			break // Saiu do mutex com LOCK ainda ativo! Vai usar abaixo
		}
		gs.RicartMu.Unlock()
		time.Sleep(50 * time.Millisecond) // Espera ocupada curta
	}

	// Neste ponto, RicartMu está LOCKED e EstadoRicart == "LIVRE"

	// Aplicar aging: se perdeu muitas vezes, elevar prioridade
	if gs.ContadorAging >= 3 {
		fmt.Printf("🔥 [AGING] Setor %s cansou de perder a vez! Prioridade elevada de %d para 2 (CRÍTICA)\n", gs.MeuSetor, prioridadeInicial)
		prioridadeInicial = 2
	}

	gs.EstadoRicart = "ESPERANDO"
	gs.MinhaPrioridade = prioridadeInicial
	gs.AlvoAtual = coordenada
	gs.AcksRecebidos = 0
	gs.MeuTempoPedido = TickLamport(gs)
	gs.RicartMu.Unlock()

	fmt.Printf("⚖️ A iniciar Ricart-Agrawala -> Prioridade: %d | Relógio: %d | Destino: %s\n", prioridadeInicial, gs.MeuTempoPedido, coordenada)

	gs.VizinhosMu.RLock()
	qtdVizinhos := len(gs.Vizinhos)
	if qtdVizinhos == 0 {
		gs.VizinhosMu.RUnlock()
		fmt.Printf("⚠️ [RICART] Sem vizinhos conectados! Alcançando consenso local instantaneamente.\n")
		VerificarConsenso(gs)
		return
	}
	fmt.Printf("📡 [RICART] Enviando requisição para %d vizinhos...\n", qtdVizinhos)

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

	// Inicia monitor de timeout para evitar deadlock permanente
	go MonitorConsensoComTimeout(gs, 15*time.Second)
}

// AvaliarPedidoVizinho avalia se deve colocar vizinho na fila ou enviar ACK
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
		fmt.Printf("🛑 Vizinho %s colocado na fila de espera.\n", msgReq.Remetente)
	} else {
		if gs.EstadoRicart == "ESPERANDO" {
			gs.ContadorAging++
			fmt.Printf("😡 Perdi a vez para %s. Contador de Aging subiu para: %d\n", msgReq.Remetente, gs.ContadorAging)
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

// ReceberAckP2P incrementa contador de ACKs recebidos
func ReceberAckP2P(gs *GlobalState) {
	gs.RicartMu.Lock()
	if gs.EstadoRicart == "ESPERANDO" {
		gs.AcksRecebidos++
	}
	gs.RicartMu.Unlock()
	VerificarConsenso(gs)
}

// VerificarConsenso verifica se todos os ACKs foram recebidos
func VerificarConsenso(gs *GlobalState) {
	gs.RicartMu.Lock()
	defer gs.RicartMu.Unlock()

	if gs.EstadoRicart != "ESPERANDO" {
		return
	}

	gs.VizinhosMu.RLock()
	vivos := len(gs.Vizinhos)
	gs.VizinhosMu.RUnlock()

	fmt.Printf("🔍 [RICART DEBUG] Estado: %s | ACKs: %d/%d | FilaEspera: %d\n", gs.EstadoRicart, gs.AcksRecebidos, vivos, len(gs.FilaDeEspera))

	if gs.AcksRecebidos >= vivos {
		gs.EstadoRicart = "USANDO"
		gs.ContadorAging = 0
		fmt.Printf("🏆 CONSENSO ALCANÇADO! Setor %s ganhou a Exclusão Mútua (ACKs: %d, Vizinhos: %d).\n", gs.MeuSetor, gs.AcksRecebidos, vivos)

		// Executa despacho em goroutine separada
		go ExecutarDespacho(gs, gs.AlvoAtual)
	}
}

// ExecutarDespacho tenta escolher um drone livre e despacha
func ExecutarDespacho(gs *GlobalState, coordenada string) {
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
	fmt.Printf("🎯 Procurando drone na rede: %d drones LIVRES encontrados\n", qtdDronesLivres)
	gs.FrotaMu.RUnlock()

	if droneEscolhido == "" {
		fmt.Println("⚠️ Alarme: Nenhum drone LIVRE encontrado na rede! A abortar requisição para evitar deadlock.")
		LiberarDrone(gs)
		return
	}

	fmt.Printf("🎯 Decisão P2P: O Drone escolhido foi o [%s] (pertence ao setor %s)\n", droneEscolhido, setorDoDrone)

	// Atualiza estado para EM_MISSAO
	gs.FrotaMu.Lock()
	if estado, ok := gs.FrotaGlobal[droneEscolhido]; ok {
		estado.Status = "EM_MISSAO"
		estado.SeenAt = time.Now().UnixNano()
		gs.FrotaGlobal[droneEscolhido] = estado
		fmt.Printf("🚁 Drone %s marcado como EM_MISSAO\n", droneEscolhido)
	}
	gs.FrotaMu.Unlock()

	if setorDoDrone == gs.MeuSetor {
		// Drone local
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
				fmt.Printf("🚀 Ordem de despacho enviada DIRETAMENTE ao drone local %s para %s! (bytes: %d)\n", droneEscolhido, coordenada, n)
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
		// Drone remoto
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
				fmt.Printf("📡 Ordem de despacho enviada VIA P2P para o setor %s comandar %s para %s! (bytes: %d)\n", setorDoDrone, droneEscolhido, coordenada, n)
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

// LiberarDrone libera a seção crítica e envia ACKs para a fila de espera
func LiberarDrone(gs *GlobalState) {
	gs.RicartMu.Lock()
	defer gs.RicartMu.Unlock()

	gs.EstadoRicart = "LIVRE"
	fmt.Printf("🔓 A libertar a exclusão mútua. A enviar ACK para %d vizinhos na fila de espera...\n", len(gs.FilaDeEspera))

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

// MonitorConsensoComTimeout monitora se o consenso foi alcançado dentro do timeout
// Se timeout expirar e ainda está ESPERANDO, reseta para LIVRE para permitir retry
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
