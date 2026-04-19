package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

// A NOVA STRUCT PADRÃO DO PBL 2
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
	Setor  string `json:"setor"` // O setor físico ao qual ele está conectado
}

var (
	// ESTADO DO DASHBOARD (Apenas Leitura/Exibição)
	mu      sync.RWMutex
	frota   = make(map[string]EstadoDrone)
	alertas = make([]string, 0) // Histórico dos últimos eventos críticos

	connMu     sync.RWMutex
	brokerConn net.Conn
)

func setConexao(conn net.Conn) {
	connMu.Lock()
	brokerConn = conn
	connMu.Unlock()
}

func getConexao() net.Conn {
	connMu.RLock()
	defer connMu.RUnlock()
	return brokerConn
}

func descartarConexao(conn net.Conn) {
	connMu.Lock()
	if brokerConn == conn {
		brokerConn = nil
	}
	connMu.Unlock()
	if conn != nil {
		conn.Close()
	}
}

func enviarMensagem(msg Mensagem) bool {
	conn := getConexao()
	if conn == nil {
		return false
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return false
	}

	_, err = fmt.Fprintf(conn, "%s\n", payload)
	if err != nil {
		descartarConexao(conn)
		return false
	}

	return true
}

func habilitarKeepAlive(conn net.Conn) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	_ = tcpConn.SetKeepAlive(true)
	_ = tcpConn.SetKeepAlivePeriod(3 * time.Second)
}

func manterConexaoComBroker(addr string) {
	for {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			fmt.Printf("⚠️ Broker offline. A tentar reconectar em 5 segundos... (%v)\n", err)
			time.Sleep(5 * time.Second)
			continue
		}
		habilitarKeepAlive(conn)

		setConexao(conn)
		fmt.Println("✅ Ligado com sucesso ao Broker do Setor!")

		// Regista-se como um Dashboard
		enviarMensagem(Mensagem{
			Tipo:      "REG",
			Remetente: "DASHBOARD_OPERADOR",
		})

		ouvirRede(conn)

		descartarConexao(conn)
		fmt.Println("❌ Ligação perdida. A iniciar reconexão...")
		time.Sleep(3 * time.Second)
	}
}

// INTERFACE COM O UTILIZADOR (CLI)
func main() {
	addrEnv := os.Getenv("INTEGRADOR_ADDR")
	if addrEnv == "" {
		addrEnv = "localhost:8083" // Porta de Clientes no Broker
	}

	go manterConexaoComBroker(addrEnv)

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("\n===========================================")
		fmt.Println("🌊 CENTRO DE COMANDO - ESTREITO DE ORMUZ 🌊")
		fmt.Println("===========================================")
		fmt.Println("[1] Ver Painel Tático (Drones e Alertas)")
		fmt.Println("[2] Despachar Drone (Ação Manual)")
		fmt.Println("[0] Sair")
		fmt.Println("===========================================")
		fmt.Print("Escolha uma opção: ")

		opcao, _ := reader.ReadString('\n')
		opcao = strings.TrimSpace(opcao)

		switch opcao {
		case "1":
			imprimirPainel()
		case "2":
			fmt.Print("📍 Digite as coordenadas para a missão (ex: 26.54,56.12): ")
			coordenada, _ := reader.ReadString('\n')
			coordenada = strings.TrimSpace(coordenada)

			conn := getConexao()
			if conn == nil {
				log.Println("⚠️ FALHA NA REDE: O Broker está inacessível!")
				break
			}

			// O Cliente pede um drone. Quem decide qual drone vai é a rede P2P!
			sucesso := enviarMensagem(Mensagem{
				Tipo:    "CMD",
				Acao:    "REQUISICAO_MANUAL",
				Posicao: coordenada,
			})

			if sucesso {
				fmt.Println("⏳ Pedido enviado ao Broker! A rede irá alocar o drone mais adequado.")
			} else {
				log.Println("⚠️ FALHA ao enviar pedido manual.")
			}

		case "0":
			fmt.Println("A desligar do sistema...")
			return
		default:
			fmt.Println("⚠️ Opção inválida.")
		}
	}
}

// Processa mensagens vindas do Broker (Apenas Leitura/Atualização de Tela)
func ouvirRede(conn net.Conn) {
	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		mensagemRaw := strings.TrimSpace(scanner.Text())
		if mensagemRaw == "" {
			continue
		}

		var msg Mensagem
		if err := json.Unmarshal([]byte(mensagemRaw), &msg); err != nil {
			continue
		}
		msg.Tipo = strings.ToUpper(strings.TrimSpace(msg.Tipo))

		mu.Lock()

		// Se receber a fofoca global da rede (Sincronização da Frota)
		if msg.Tipo == "GOSSIP" && msg.Frota != nil {
			// Atualiza a visão local da frota com os dados mais recentes do Broker
			for id, estado := range msg.Frota {
				frota[id] = estado
			}
		}

		// Se receber um evento crítico do mar
		if msg.Tipo == "EVT" && msg.Acao == "ALERTA" {
			alertaTexto := fmt.Sprintf("[%s] %s em %s", msg.Remetente, msg.Valor, msg.Posicao)

			// Mantém apenas os últimos 5 alertas
			alertas = append(alertas, alertaTexto)
			if len(alertas) > 5 {
				alertas = alertas[1:] // Remove o mais antigo
			}
		}

		mu.Unlock()
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("⚠️ Leitura da ligação com o Broker falhou: %v\n", err)
	}
}

// Imprime a frota e os alertas mais recentes
func imprimirPainel() {
	mu.RLock()
	defer mu.RUnlock()

	fmt.Println("\n🚁 === STATUS DA FROTA DE DRONES ===")
	if len(frota) == 0 {
		fmt.Println("Nenhum drone detetado na rede neste momento.")
	} else {
		for id, drone := range frota {
			icone := "🟢"
			if drone.Status == "EM_MISSAO" {
				icone = "🔴"
			} else if drone.Status == "DESCONECTADO" {
				icone = "❌"
			}
			fmt.Printf("%s [%s] -> Status: %s | Operando em: %s\n", icone, id, drone.Status, drone.Setor)
		}
	}

	fmt.Println("\n🚨 === ÚLTIMOS ALERTAS CRÍTICOS ===")
	if len(alertas) == 0 {
		fmt.Println("Mar calmo. Nenhum alerta crítico detetado.")
	} else {
		for i := len(alertas) - 1; i >= 0; i-- {
			fmt.Printf("- %s\n", alertas[i])
		}
	}
	fmt.Println("===========================================")
}
