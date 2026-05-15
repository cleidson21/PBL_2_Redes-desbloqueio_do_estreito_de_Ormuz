package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"time"
)

// ListenP2P aceita mensagens entre vizinhos e mantém a malha local sincronizada.
func ListenP2P(gs *GlobalState) {
	listener, err := net.Listen("tcp", ":48084")
	if err != nil {
		fmt.Printf("❌ [%s] Erro ao iniciar porta P2P 48084: %v\n", gs.MeuNamespace, err)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		HabilitarKeepAlive(conn)
		go ManipularMensagemP2P(gs, conn)
	}
}

// ConectarAosVizinhos inicia tentativas de conexão persistentes com todos os vizinhos configurados.
func ConectarAosVizinhos(gs *GlobalState, peers string) {
	if peers == "" {
		return
	}

	peerList := parseAddressList(peers)
	for _, peerAddr := range peerList {
		go reconectarAVizinho(gs, peerAddr)
	}
}

func reconectarAVizinho(gs *GlobalState, addr string) {
	for {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		HabilitarKeepAlive(conn)

		msgHello := Mensagem{
			Tipo:      "P2P_HELLO",
			Remetente: gs.MeuSetor,
			Relogio:   TickLamport(gs),
		}
		payload, _ := json.Marshal(msgHello)
		fmt.Fprintf(conn, "%s\n", payload)

		ManipularMensagemP2P(gs, conn)
		time.Sleep(3 * time.Second)
	}
}

// ManipularMensagemP2P processa sincronização P2P, gossip e encaminhamento entre setores.
func ManipularMensagemP2P(gs *GlobalState, conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	var vizinhoID string

	for scanner.Scan() {
		var msg Mensagem
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}

		SyncLamport(gs, msg.Relogio)

		switch msg.Tipo {
		case "P2P_HELLO":
			vizinhoID = msg.Remetente
			gs.VizinhosMu.Lock()
			gs.Vizinhos[vizinhoID] = conn
			gs.VizinhosMu.Unlock()
			fmt.Printf("🤝 [%s] Vizinho registado na malha: %s\n", gs.MeuNamespace, vizinhoID)

		case "GOSSIP":
			gs.FrotaMu.Lock()
			for idDrone, estadoDrone := range msg.Frota {
				if estadoAtual, existe := gs.FrotaGlobal[idDrone]; !existe || estadoDrone.SeenAt >= estadoAtual.SeenAt {
					if estadoDrone.SeenAt == 0 {
						estadoDrone.SeenAt = time.Now().UnixNano()
					}
					gs.FrotaGlobal[idDrone] = estadoDrone
				}
			}
			gs.FrotaMu.Unlock()

		case "P2P_REQ":
			AvaliarPedidoVizinho(gs, msg, conn)

		case "ACK":
			ReceberAckP2P(gs)

		case "P2P_CMD":
			fmt.Printf("📥 Recebida ordem P2P para despachar o drone local [%s]!\n", msg.Destino)
			gs.DronesMu.RLock()
			connDrone, ok := gs.DronesLocais[msg.Destino]
			gs.DronesMu.RUnlock()

			if ok {
				cmdParaDrone := Mensagem{
					Tipo:    "CMD",
					Acao:    msg.Acao,
					Posicao: msg.Posicao,
				}
				payload, _ := json.Marshal(cmdParaDrone)
				fmt.Fprintf(connDrone, "%s\n", payload)
			}
		}
	}

	if vizinhoID != "" {
		gs.VizinhosMu.Lock()
		delete(gs.Vizinhos, vizinhoID)
		gs.VizinhosMu.Unlock()
		fmt.Printf("❌ [%s] Vizinho %s morreu e foi removido da lista.\n", gs.MeuNamespace, vizinhoID)

		gs.FrotaMu.Lock()
		for idDrone, estadoDrone := range gs.FrotaGlobal {
			if estadoDrone.Setor == vizinhoID {
				delete(gs.FrotaGlobal, idDrone)
			}
		}
		gs.FrotaMu.Unlock()

		VerificarConsenso(gs)
	}
	conn.Close()
}

// RotinaGossip replica periodicamente o estado da frota para vizinhos e dashboards.
func RotinaGossip(gs *GlobalState) {
	for {
		time.Sleep(5 * time.Second)

		gs.FrotaMu.RLock()
		if len(gs.FrotaGlobal) == 0 {
			gs.FrotaMu.RUnlock()
			continue
		}
		copiaFrota := make(map[string]EstadoDrone)
		for k, v := range gs.FrotaGlobal {
			copiaFrota[k] = v
		}
		gs.FrotaMu.RUnlock()

		msgGossip := Mensagem{
			Tipo:      "GOSSIP",
			Remetente: gs.MeuSetor,
			Relogio:   TickLamport(gs),
			Frota:     copiaFrota,
		}
		payload, _ := json.Marshal(msgGossip)

		gs.VizinhosMu.RLock()
		for _, conn := range gs.Vizinhos {
			fmt.Fprintf(conn, "%s\n", payload)
		}
		gs.VizinhosMu.RUnlock()

		gs.DashboardsMu.RLock()
		for conn := range gs.Dashboards {
			fmt.Fprintf(conn, "%s\n", payload)
		}
		gs.DashboardsMu.RUnlock()
	}
}
