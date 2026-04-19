package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// A NOVA STRUCT PADRÃO
type Mensagem struct {
	Tipo      string                 `json:"tipo"`
	Remetente string                 `json:"remetente,omitempty"`
	Destino   string                 `json:"destino,omitempty"`
	Relogio   int                    `json:"relogio,omitempty"`
	Acao      string                 `json:"acao,omitempty"`
	Valor     string                 `json:"valor,omitempty"`
	Posicao   string                 `json:"posicao,omitempty"`
	Frota     map[string]EstadoDrone `json:"frota,omitempty"`
}

type EstadoDrone struct {
	Status string `json:"status"`
	Setor  string `json:"setor"`
}

var (
	// IDENTIDADE DO BROKER
	meuSetor = os.Getenv("MEU_SETOR") // Ex: "SETOR_SUL"

	// ALGORITMO DE LAMPORT
	muLamport sync.Mutex
	relogio   int = 0

	// CONEXÕES FÍSICAS LOCAIS (Sensores e Drones conectados DIRETAMENTE a este Broker)
	muDrones sync.RWMutex
	drones   = make(map[string]net.Conn) // Drones ligados neste setor

	muClientes sync.RWMutex
	clientes   = make(map[net.Conn]bool) // Dashboards ligados neste setor

	// REDE P2P (Outros Brokers)
	muVizinhos sync.RWMutex
	vizinhos   = make(map[string]net.Conn) // Ex: "SETOR_NORTE": conn

	// ESTADO DISTRIBUÍDO (A fofoca global)
	muFrota     sync.RWMutex
	frotaGlobal = make(map[string]EstadoDrone)
)

func main() {
	if meuSetor == "" {
		meuSetor = "SETOR_DESCONHECIDO"
	}

	fmt.Printf("🚀 Broker P2P Iniciado: [%s]\n", meuSetor)

	// Inicia as escutas das pontas (Igual ao PBL 1)
	go listenSensoresTLM()  // Porta 8080
	go listenRadarTCP()     // Porta 8081
	go listenDrones()       // Porta 8082
	go listenDashboardTCP() // Porta 8083

	// INICIA A REDE P2P (A Mágica do PBL 2)
	go listenP2P() // Porta 8084 (Escuta outros brokers)

	// Dá um tempo para todos os contêineres subirem antes de tentar conectar
	time.Sleep(3 * time.Second)
	go conectarAosVizinhos()

	// Inicia a rotina de fofoca (Gossip Protocol)
	go rotinaGossip()

	select {}
}

// Incrementa o relógio local ao fazer um evento interno
func tickLamport() int {
	muLamport.Lock()
	defer muLamport.Unlock()
	relogio++
	return relogio
}

// Atualiza o relógio local ao receber uma mensagem de fora
func syncLamport(relogioRecebido int) {
	muLamport.Lock()
	defer muLamport.Unlock()
	if relogioRecebido > relogio {
		relogio = relogioRecebido
	}
	relogio++
}

// Ouve chamadas de outros Brokers na porta 8084
func listenP2P() {
	listener, err := net.Listen("tcp", ":8084")
	if err != nil {
		fmt.Printf("❌ [%s] Erro ao iniciar porta P2P 8084: %v\n", meuSetor, err)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		// Quando um Broker vizinho se conecta, iniciamos a leitura
		go manipularMensagemP2P(conn)
	}
}

// Disca ativamente para os vizinhos definidos na variável PEERS
func conectarAosVizinhos() {
	peersEnv := os.Getenv("PEERS") // Ex: "172.18.0.3:8084,172.18.0.4:8084"
	if peersEnv == "" {
		fmt.Printf("ℹ️ [%s] Nenhum vizinho definido (Modo Isolado).\n", meuSetor)
		return
	}

	listaPeers := strings.Split(peersEnv, ",")

	for _, peerAddr := range listaPeers {
		peerAddr = strings.TrimSpace(peerAddr)
		go func(addr string) {
			for {
				conn, err := net.Dial("tcp", addr)
				if err != nil {
					time.Sleep(5 * time.Second) // Tenta reconectar a cada 5s se o vizinho cair
					continue
				}

				fmt.Printf("🔗 [%s] Conectado ao vizinho em: %s\n", meuSetor, addr)

				// Envia um "Olá" dizendo quem eu sou para o vizinho salvar no mapa dele
				msgHello := Mensagem{
					Tipo:      "P2P_HELLO",
					Remetente: meuSetor,
					Relogio:   tickLamport(),
				}
				payload, _ := json.Marshal(msgHello)
				fmt.Fprintf(conn, "%s\n", payload)

				// Fica ouvindo o que esse vizinho tem a dizer
				manipularMensagemP2P(conn)

				fmt.Printf("⚠️ [%s] Conexão P2P perdida com %s. Tentando reconectar...\n", meuSetor, addr)
				time.Sleep(3 * time.Second)
			}
		}(peerAddr)
	}
}
