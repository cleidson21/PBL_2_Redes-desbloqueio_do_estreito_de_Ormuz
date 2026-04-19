package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// ==============================================================================
// ETAPA 1: FUNDAÇÕES E ESTRUTURA DE DADOS
// ==============================================================================

type Mensagem struct {
	Tipo       string                 `json:"tipo"`
	Remetente  string                 `json:"remetente,omitempty"`
	Destino    string                 `json:"destino,omitempty"`
	Relogio    int                    `json:"relogio,omitempty"`
	Prioridade int                    `json:"prioridade,omitempty"`
	Acao       string                 `json:"acao,omitempty"`
	Valor      string                 `json:"valor,omitempty"`
	Posicao    string                 `json:"posicao,omitempty"`
	Frota      map[string]EstadoDrone `json:"frota,omitempty"`
}

type EstadoDrone struct {
	Status string `json:"status"`
	Setor  string `json:"setor"`
}

var (
	meuSetor     = os.Getenv("MEU_SETOR")
	prefixoOrmuz = "ORMUZ"
	meuNamespace = ""

	muLamport sync.Mutex
	relogio   int = 0

	muRadares    sync.RWMutex
	radares      = make(map[string]net.Conn)
	muSensores   sync.RWMutex
	sensores     = make(map[string]*net.UDPAddr)
	muDrones     sync.RWMutex
	dronesLocais = make(map[string]net.Conn)
	muDashboards sync.RWMutex
	dashboards   = make(map[net.Conn]bool)

	muVizinhos  sync.RWMutex
	vizinhos    = make(map[string]net.Conn)
	muFrota     sync.RWMutex
	frotaGlobal = make(map[string]EstadoDrone)

	muRicart        sync.Mutex
	estadoRicart    = "LIVRE"
	meuTempoPedido  int
	minhaPrioridade int
	contadorAging   = 0
	acksRecebidos   = 0
	filaDeEspera    []Mensagem
	alvoAtual       string
)

func init() {
	if meuSetor == "" {
		meuSetor = "DESCONHECIDO"
	}
	meuNamespace = fmt.Sprintf("%s/%s", prefixoOrmuz, meuSetor)
}

func tickLamport() int {
	muLamport.Lock()
	defer muLamport.Unlock()
	relogio++
	return relogio
}

func syncLamport(relogioRecebido int) {
	muLamport.Lock()
	defer muLamport.Unlock()
	if relogioRecebido > relogio {
		relogio = relogioRecebido
	}
	relogio++
}

// ==============================================================================
// ETAPA 4 e 5: EXCLUSÃO MÚTUA E DESPACHO FÍSICO
// ==============================================================================

func iniciarRequisicaoDrone(prioridadeInicial int, coordenada string) {
	muRicart.Lock()
	if estadoRicart != "LIVRE" {
		muRicart.Unlock()
		fmt.Println("⚠️ Já existe uma requisição em andamento neste setor. A aguardar...")
		return
	}

	if contadorAging >= 3 {
		fmt.Printf("🔥 [AGING] Setor %s cansou de perder a vez! Prioridade elevada de %d para 2 (CRÍTICA)\n", meuSetor, prioridadeInicial)
		prioridadeInicial = 2
	}

	estadoRicart = "ESPERANDO"
	minhaPrioridade = prioridadeInicial
	alvoAtual = coordenada
	acksRecebidos = 0
	meuTempoPedido = tickLamport()
	muRicart.Unlock()

	fmt.Printf("⚖️ A iniciar Ricart-Agrawala -> Prioridade: %d | Relógio: %d | Destino: %s\n", minhaPrioridade, meuTempoPedido, coordenada)

	muVizinhos.RLock()
	qtdVizinhos := len(vizinhos)
	if qtdVizinhos == 0 {
		muVizinhos.RUnlock()
		verificarConsenso()
		return
	}

	msgReq := Mensagem{
		Tipo:       "P2P_REQ",
		Remetente:  meuSetor,
		Relogio:    meuTempoPedido,
		Prioridade: minhaPrioridade,
	}
	payload, _ := json.Marshal(msgReq)

	for _, conn := range vizinhos {
		fmt.Fprintf(conn, "%s\n", payload)
	}
	muVizinhos.RUnlock()
}

func avaliarPedidoVizinho(msgReq Mensagem, connVizinho net.Conn) {
	muRicart.Lock()
	defer muRicart.Unlock()

	devoAtrasar := false

	if estadoRicart == "USANDO" {
		devoAtrasar = true
	} else if estadoRicart == "ESPERANDO" {
		if minhaPrioridade > msgReq.Prioridade {
			devoAtrasar = true
		} else if minhaPrioridade == msgReq.Prioridade {
			if meuTempoPedido < msgReq.Relogio {
				devoAtrasar = true
			} else if meuTempoPedido == msgReq.Relogio {
				if meuSetor < msgReq.Remetente {
					devoAtrasar = true
				}
			}
		}
	}

	if devoAtrasar {
		filaDeEspera = append(filaDeEspera, msgReq)
		fmt.Printf("🛑 Vizinho %s colocado na fila de espera.\n", msgReq.Remetente)
	} else {
		if estadoRicart == "ESPERANDO" {
			contadorAging++
			fmt.Printf("😡 Perdi a vez para %s. Contador de Aging subiu para: %d\n", msgReq.Remetente, contadorAging)
		}
		ackMsg := Mensagem{
			Tipo:      "ACK",
			Remetente: meuSetor,
			Destino:   msgReq.Remetente,
		}
		payload, _ := json.Marshal(ackMsg)
		fmt.Fprintf(connVizinho, "%s\n", payload)
	}
}

func receberAckP2P() {
	muRicart.Lock()
	if estadoRicart == "ESPERANDO" {
		acksRecebidos++
	}
	muRicart.Unlock()
	verificarConsenso()
}

func verificarConsenso() {
	muRicart.Lock()
	defer muRicart.Unlock()

	if estadoRicart != "ESPERANDO" {
		return
	}

	muVizinhos.RLock()
	vivos := len(vizinhos)
	muVizinhos.RUnlock()

	if acksRecebidos >= vivos {
		estadoRicart = "USANDO"
		contadorAging = 0
		fmt.Printf("🏆 CONSENSO ALCANÇADO! Setor %s ganhou a Exclusão Mútua.\n", meuSetor)

		// [ETAPA 5] Dispara a execução real do despacho!
		go executarDespacho(alvoAtual)
	}
}

// [ETAPA 5] O Roteamento Físico da Decisão Lógica
func executarDespacho(coordenada string) {
	var droneEscolhido string
	var setorDoDrone string

	muFrota.RLock()
	for id, estado := range frotaGlobal {
		if estado.Status == "LIVRE" {
			droneEscolhido = id
			setorDoDrone = estado.Setor
			break
		}
	}
	muFrota.RUnlock()

	if droneEscolhido == "" {
		fmt.Println("⚠️ Alarme: Nenhum drone LIVRE encontrado na rede! A abortar requisição para evitar deadlock.")
		liberarDrone()
		return
	}

	fmt.Printf("🎯 Decisão P2P: O Drone escolhido foi o [%s] (pertence ao setor %s)\n", droneEscolhido, setorDoDrone)

	// Atualiza o estado logicamente de forma otimista para os outros não o tentarem usar
	muFrota.Lock()
	if estado, ok := frotaGlobal[droneEscolhido]; ok {
		estado.Status = "EM_MISSAO"
		frotaGlobal[droneEscolhido] = estado
	}
	muFrota.Unlock()

	if setorDoDrone == meuSetor {
		// O Drone está fisicamente ligado a MIM. Mando direto no TCP dele.
		muDrones.RLock()
		connDrone, ok := dronesLocais[droneEscolhido]
		muDrones.RUnlock()

		if ok {
			cmdMsg := Mensagem{
				Tipo:    "CMD",
				Acao:    "DESPACHAR",
				Posicao: coordenada,
			}
			payload, _ := json.Marshal(cmdMsg)
			fmt.Fprintf(connDrone, "%s\n", payload)
			fmt.Printf("🚀 Ordem de despacho enviada DIRETAMENTE ao drone local %s!\n", droneEscolhido)
		}
	} else {
		// O Drone está fisicamente noutro setor. Peço ao vizinho para dar a ordem.
		muVizinhos.RLock()
		connVizinho, ok := vizinhos[setorDoDrone]
		muVizinhos.RUnlock()

		if ok {
			cmdMsg := Mensagem{
				Tipo:    "P2P_CMD",
				Destino: droneEscolhido,
				Acao:    "DESPACHAR",
				Posicao: coordenada,
			}
			payload, _ := json.Marshal(cmdMsg)
			fmt.Fprintf(connVizinho, "%s\n", payload)
			fmt.Printf("📡 Ordem de despacho enviada VIA P2P para o setor %s comandar o drone!\n", setorDoDrone)
		}
	}

	// Liberta a exclusão mútua assim que a ordem é dada.
	// O drone fará o voo de forma autónoma sem prender a rede!
	liberarDrone()
}

func liberarDrone() {
	muRicart.Lock()
	defer muRicart.Unlock()

	estadoRicart = "LIVRE"
	fmt.Printf("🔓 A libertar a exclusão mútua. A enviar ACK para %d vizinhos na fila de espera...\n", len(filaDeEspera))

	muVizinhos.RLock()
	for _, reqAntiga := range filaDeEspera {
		if conn, existe := vizinhos[reqAntiga.Remetente]; existe {
			ackMsg := Mensagem{
				Tipo:      "ACK",
				Remetente: meuSetor,
				Destino:   reqAntiga.Remetente,
			}
			payload, _ := json.Marshal(ackMsg)
			fmt.Fprintf(conn, "%s\n", payload)
		}
	}
	muVizinhos.RUnlock()

	filaDeEspera = nil
}

// ==============================================================================
// ETAPA 2: A MALHA P2P E FOFOCA
// ==============================================================================

func listenP2P() {
	listener, err := net.Listen("tcp", ":8084")
	if err != nil {
		fmt.Printf("❌ [%s] Erro ao iniciar porta P2P 8084: %v\n", meuNamespace, err)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go manipularMensagemP2P(conn)
	}
}

func conectarAosVizinhos() {
	peersEnv := os.Getenv("PEERS")
	if peersEnv == "" {
		return
	}

	listaPeers := strings.Split(peersEnv, ",")
	for _, peerAddr := range listaPeers {
		peerAddr = strings.TrimSpace(peerAddr)
		if peerAddr == "" {
			continue
		}

		go func(addr string) {
			for {
				conn, err := net.Dial("tcp", addr)
				if err != nil {
					time.Sleep(5 * time.Second)
					continue
				}

				msgHello := Mensagem{
					Tipo:      "P2P_HELLO",
					Remetente: meuSetor,
					Relogio:   tickLamport(),
				}
				payload, _ := json.Marshal(msgHello)
				fmt.Fprintf(conn, "%s\n", payload)

				manipularMensagemP2P(conn)
				time.Sleep(3 * time.Second)
			}
		}(peerAddr)
	}
}

func manipularMensagemP2P(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	var vizinhoID string

	for scanner.Scan() {
		var msg Mensagem
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}

		syncLamport(msg.Relogio)

		switch msg.Tipo {
		case "P2P_HELLO":
			vizinhoID = msg.Remetente
			muVizinhos.Lock()
			vizinhos[vizinhoID] = conn
			muVizinhos.Unlock()
			fmt.Printf("🤝 [%s] Vizinho registado na malha: %s\n", meuNamespace, vizinhoID)

		case "GOSSIP":
			muFrota.Lock()
			for idDrone, estadoDrone := range msg.Frota {
				frotaGlobal[idDrone] = estadoDrone
			}
			muFrota.Unlock()

		case "P2P_REQ":
			avaliarPedidoVizinho(msg, conn)

		case "ACK":
			receberAckP2P()

		// [ETAPA 5] Se o vizinho ganhou o consenso e me mandou dar a ordem ao MEU drone
		case "P2P_CMD":
			fmt.Printf("📥 Recebida ordem P2P para despachar o drone local [%s]!\n", msg.Destino)
			muDrones.RLock()
			connDrone, ok := dronesLocais[msg.Destino]
			muDrones.RUnlock()

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
		muVizinhos.Lock()
		delete(vizinhos, vizinhoID)
		muVizinhos.Unlock()
		fmt.Printf("❌ [%s] Vizinho %s morreu e foi removido da lista.\n", meuNamespace, vizinhoID)

		// [ETAPA 5] DETETOR DE FALHAS: Recalcula se o vizinho morto era o único ACK que faltava
		verificarConsenso()
	}
	conn.Close()
}

func rotinaGossip() {
	for {
		time.Sleep(5 * time.Second)

		muFrota.RLock()
		if len(frotaGlobal) == 0 {
			muFrota.RUnlock()
			continue
		}
		copiaFrota := make(map[string]EstadoDrone)
		for k, v := range frotaGlobal {
			copiaFrota[k] = v
		}
		muFrota.RUnlock()

		msgGossip := Mensagem{
			Tipo:      "GOSSIP",
			Remetente: meuSetor,
			Relogio:   tickLamport(),
			Frota:     copiaFrota,
		}
		payload, _ := json.Marshal(msgGossip)

		muVizinhos.RLock()
		for _, conn := range vizinhos {
			fmt.Fprintf(conn, "%s\n", payload)
		}
		muVizinhos.RUnlock()

		muDashboards.RLock()
		for conn := range dashboards {
			fmt.Fprintf(conn, "%s\n", payload)
		}
		muDashboards.RUnlock()
	}
}

// ==============================================================================
// ETAPA 3: CONEXÕES LOCAIS E PERIFERIA
// ==============================================================================

func habilitarKeepAlive(conn net.Conn) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	_ = tcpConn.SetKeepAlive(true)
	_ = tcpConn.SetKeepAlivePeriod(3 * time.Second)
}

func enriquecerIdentidade(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Sprintf("%s/DESCONHECIDO", meuNamespace)
	}
	if !strings.HasPrefix(id, prefixoOrmuz) {
		return fmt.Sprintf("%s/%s", meuNamespace, id)
	}
	return id
}

func atualizarDashboards(msg Mensagem) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}

	muDashboards.RLock()
	defer muDashboards.RUnlock()
	for conn := range dashboards {
		fmt.Fprintf(conn, "%s\n", payload)
	}
}

func listenSensoresTLM() {
	addr, _ := net.ResolveUDPAddr("udp", ":8080")
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return
	}
	defer conn.Close()

	buffer := make([]byte, 1024)
	for {
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			continue
		}
		var msg Mensagem
		if err := json.Unmarshal(buffer[:n], &msg); err != nil {
			continue
		}
		msg.Remetente = enriquecerIdentidade(msg.Remetente)
	}
}

func listenRadarTCP() {
	listener, err := net.Listen("tcp", ":8081")
	if err != nil {
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		habilitarKeepAlive(conn)

		go func(c net.Conn) {
			defer c.Close()
			scanner := bufio.NewScanner(c)
			for scanner.Scan() {
				var msg Mensagem
				if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
					continue
				}

				msg.Remetente = enriquecerIdentidade(msg.Remetente)
				syncLamport(msg.Relogio)

				if msg.Tipo == "EVT" && msg.Acao == "ALERTA" {
					fmt.Printf("🚨 ALERTA CRÍTICO DETETADO [%s]: %s em %s\n", msg.Remetente, msg.Valor, msg.Posicao)
					atualizarDashboards(msg)
					iniciarRequisicaoDrone(2, msg.Posicao)
				}
			}
		}(conn)
	}
}

func listenDrones() {
	listener, err := net.Listen("tcp", ":8082")
	if err != nil {
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		habilitarKeepAlive(conn)

		go func(c net.Conn) {
			scanner := bufio.NewScanner(c)
			var droneID string

			for scanner.Scan() {
				var msg Mensagem
				if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
					continue
				}

				msg.Remetente = enriquecerIdentidade(msg.Remetente)
				syncLamport(msg.Relogio)

				if msg.Tipo == "REG" {
					droneID = msg.Remetente

					muDrones.Lock()
					dronesLocais[droneID] = c
					muDrones.Unlock()

					muFrota.Lock()
					frotaGlobal[droneID] = EstadoDrone{Status: "LIVRE", Setor: meuSetor}
					muFrota.Unlock()

					fmt.Printf("🚁 [%s] Drone registado na base local!\n", droneID)

				} else if msg.Tipo == "ACK" {
					muFrota.Lock()
					if estado, existe := frotaGlobal[msg.Remetente]; existe {
						estado.Status = msg.Valor
						frotaGlobal[msg.Remetente] = estado
					}
					muFrota.Unlock()

					atualizarDashboards(msg)
				}
			}

			if droneID != "" {
				muDrones.Lock()
				delete(dronesLocais, droneID)
				muDrones.Unlock()

				muFrota.Lock()
				if estado, existe := frotaGlobal[droneID]; existe {
					estado.Status = "DESCONECTADO"
					frotaGlobal[droneID] = estado
				}
				muFrota.Unlock()
			}
			c.Close()
		}(conn)
	}
}

func listenDashboardTCP() {
	listener, err := net.Listen("tcp", ":8083")
	if err != nil {
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		habilitarKeepAlive(conn)

		muDashboards.Lock()
		dashboards[conn] = true
		muDashboards.Unlock()

		go func(c net.Conn) {
			defer func() {
				muDashboards.Lock()
				delete(dashboards, c)
				muDashboards.Unlock()
				c.Close()
			}()

			scanner := bufio.NewScanner(c)
			for scanner.Scan() {
				var msg Mensagem
				if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
					continue
				}

				msg.Remetente = enriquecerIdentidade(msg.Remetente)
				syncLamport(msg.Relogio)

				if msg.Tipo == "CMD" && msg.Acao == "REQUISICAO_MANUAL" {
					fmt.Printf("👨‍💻 Operador solicitou inspeção manual para: %s\n", msg.Posicao)
					iniciarRequisicaoDrone(1, msg.Posicao)
				}
			}
		}(conn)
	}
}

// ==============================================================================
// MAIN
// ==============================================================================

func main() {
	fmt.Printf("🚀 Servidor de Setor Iniciado: [%s]\n", meuNamespace)
	fmt.Printf("🕒 Relógio Lógico Lamport inicializado em: %d\n", relogio)
	fmt.Println("==================================================")

	go listenP2P()
	go listenSensoresTLM()
	go listenRadarTCP()
	go listenDrones()
	go listenDashboardTCP()

	time.Sleep(3 * time.Second)
	go conectarAosVizinhos()
	go rotinaGossip()

	select {}
}
