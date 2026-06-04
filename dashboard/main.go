package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// Mensagem representa o contrato JSON trocado com o broker.
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

// EstadoDrone descreve o snapshot mais recente de um drone na frota.
type EstadoDrone struct {
	Status string `json:"status"`
	Setor  string `json:"setor"`
}

// Requisicao representa o histórico recente de exclusão mútua exibido no painel.
type Requisicao struct {
	ID         string
	Prioridade int
	Lamport    int
	Status     string
	Timestamp  int64
}

var (
	mu          sync.RWMutex
	frota       = make(map[string]EstadoDrone)
	alertas     = make([]string, 0)
	requisicoes = make([]Requisicao, 0)
	telemetrias = make([]string, 0)

	connMu     sync.RWMutex
	brokerConn net.Conn

	uiMu        sync.RWMutex
	modoComando bool
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

func manterConexaoComBroker(addrVars string) {
	listaServidores := strings.Split(addrVars, ",")
	idxServidor := 0

	for {
		addr := strings.TrimSpace(listaServidores[idxServidor])
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			// Rotaciona entre brokers para evitar dependência de um único ponto de falha.
			idxServidor = (idxServidor + 1) % len(listaServidores)
			time.Sleep(3 * time.Second)
			continue
		}
		habilitarKeepAlive(conn)
		setConexao(conn)

		sucesso := enviarMensagem(Mensagem{
			Tipo:      "REG",
			Remetente: "DASHBOARD_OPERADOR",
		})
		if !sucesso {
			descartarConexao(conn)
			idxServidor = (idxServidor + 1) % len(listaServidores)
			time.Sleep(3 * time.Second)
			continue
		}

		ouvirRede(conn)

		descartarConexao(conn)
		idxServidor = (idxServidor + 1) % len(listaServidores)
		time.Sleep(3 * time.Second)
	}
}

func main() {
	addrVars := os.Getenv("SERVER_ADDRS")
	if addrVars == "" {
		addrVars = "localhost:48083"
	}

	go manterConexaoComBroker(addrVars)

	go renderizadorAutomatico()

	reader := bufio.NewReader(os.Stdin)

	for {
		reader.ReadString('\n')

		uiMu.Lock()
		modoComando = true
		uiMu.Unlock()

		limparTela()
		fmt.Println("===========================================")
		fmt.Println("🌊 MODO DE COMANDO - ESTREITO DE ORMUZ 🌊")
		fmt.Println("===========================================")
		fmt.Println("[1] Voltar ao Painel Tático Automático")
		fmt.Println("[2] Despachar Drone (Ação Manual)")
		fmt.Println("[0] Sair do Sistema")
		fmt.Println("===========================================")
		fmt.Print("Escolha uma opção: ")

		opcao, _ := reader.ReadString('\n')
		opcao = strings.TrimSpace(opcao)

		switch opcao {
		case "1":
		case "2":
			fmt.Print("\n📍 Digite as coordenadas para a missão (ex: 26.54,56.12): ")
			coordenada, _ := reader.ReadString('\n')
			coordenada = strings.TrimSpace(coordenada)

			conn := getConexao()
			if conn == nil {
				fmt.Println("⚠️ FALHA NA REDE: O Broker está inacessível!")
			} else {
				sucesso := enviarMensagem(Mensagem{
					Tipo:    "CMD",
					Acao:    "REQUISICAO_MANUAL",
					Posicao: coordenada,
				})
				if sucesso {
					fmt.Println("✅ Pedido enviado ao Broker! A rede irá alocar o drone.")
				} else {
					fmt.Println("⚠️ FALHA ao enviar pedido manual.")
				}
			}
			fmt.Print("\nPressione [ENTER] para voltar ao Painel Tático...")
			reader.ReadString('\n')
		case "0":
			limparTela()
			fmt.Println("🔌 A desligar do sistema...")
			return
		}

		uiMu.Lock()
		modoComando = false
		uiMu.Unlock()
	}
}

func renderizadorAutomatico() {
	for {
		uiMu.RLock()
		isComando := modoComando
		uiMu.RUnlock()

		if !isComando {
			// Evita sobrescrever a entrada interativa enquanto o operador está digitando.
			limparTela()
			imprimirPainel()
		}
		time.Sleep(1 * time.Second)
	}
}

func limparTela() {
	fmt.Print("\033[H\033[2J")
}

func ouvirRede(conn net.Conn) {
	// O protocolo chega como uma linha JSON por mensagem; entradas inválidas são descartadas sem interromper o fluxo.
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
		if msg.Tipo == "GOSSIP" && msg.Frota != nil {
			for id, estado := range msg.Frota {
				frota[id] = estado
			}
		}

		if msg.Tipo == "TLM" {
			telemetriaTexto := fmt.Sprintf("[%s] Telemetria: %s", msg.Remetente, msg.Valor)
			telemetrias = append(telemetrias, telemetriaTexto)
			if len(telemetrias) > 5 {
				telemetrias = telemetrias[1:]
			}
		}

		if msg.Tipo == "EVT" && msg.Acao == "ALERTA" {
			alertaTexto := fmt.Sprintf("[%s] %s em %s", msg.Remetente, msg.Valor, msg.Posicao)
			alertas = append(alertas, alertaTexto)
			if len(alertas) > 5 {
				alertas = alertas[1:]
			}
		}

		if msg.Tipo == "REQ_UPDATE" {
			requisicao := Requisicao{
				ID:         msg.Remetente,
				Prioridade: msg.Prioridade,
				Lamport:    msg.Relogio,
				Status:     strings.ToUpper(strings.TrimSpace(msg.Acao)),
				Timestamp:  time.Now().UnixNano(),
			}

			for i := range requisicoes {
				if requisicoes[i].ID == requisicao.ID {
					requisicoes = append(requisicoes[:i], requisicoes[i+1:]...)
					break
				}
			}

			requisicoes = append(requisicoes, requisicao)
			if len(requisicoes) > 20 {
				requisicoes = requisicoes[len(requisicoes)-20:]
			}
		}
		mu.Unlock()
	}
}

func imprimirPainel() {
	mu.RLock()
	defer mu.RUnlock()

	statusRede := "🟢 ONLINE"
	if getConexao() == nil {
		statusRede = "🔴 OFFLINE (Procurando servidor...)"
	}

	fmt.Println("======================================================")
	fmt.Printf(" 🌊 PAINEL TÁTICO - ESTREITO DE ORMUZ | REDE: %s \n", statusRede)
	fmt.Println("======================================================")

	fmt.Println("\n🚁 === STATUS DA FROTA DE DRONES ===")
	if len(frota) == 0 {
		fmt.Println("  Nenhum drone detetado na rede neste momento.")
	} else {
		for id, drone := range frota {
			icone := "🟢"
			if drone.Status == "EM_MISSAO" {
				icone = "🔴"
			} else if drone.Status == "DESCONECTADO" {
				icone = "❌"
			}
			fmt.Printf("  %s [%s] -> %s | Base: %s\n", icone, id, drone.Status, drone.Setor)
		}
	}

	fmt.Println("\n🚨 === ÚLTIMOS ALERTAS CRÍTICOS ===")
	if len(alertas) == 0 {
		fmt.Println("  ✅ Mar calmo. Nenhum evento crítico detetado.")
	} else {
		for i := len(alertas) - 1; i >= 0; i-- {
			fmt.Printf("  ⚠️ %s\n", alertas[i])
		}
	}

	fmt.Println("\n📜 === HISTÓRICO DE EXCLUSÃO MÚTUA ===")
	if len(requisicoes) == 0 {
		fmt.Println("  Nenhuma requisição recente.")
	} else {
		sort.Slice(requisicoes, func(i, j int) bool {
			if requisicoes[i].Lamport == requisicoes[j].Lamport {
				return requisicoes[i].Prioridade > requisicoes[j].Prioridade
			}
			return requisicoes[i].Lamport > requisicoes[j].Lamport
		})

		fmt.Printf("  %-28s %-5s %-8s %-18s %-19s\n", "ID", "Prio", "Lamport", "Status", "Hora de Execução")
		fmt.Println("  -----------------------------------------------------------------------------------")
		for i := 0; i < len(requisicoes); i++ {
			req := requisicoes[i]
			instante := time.Unix(0, req.Timestamp).Format("02/01 15:04:05")

			statusFormatado := req.Status
			if strings.Contains(req.Status, "EXECUTED") {
				statusFormatado = "✅ " + req.Status
			} else if strings.Contains(req.Status, "ENQUEUED") {
				statusFormatado = "🕒 " + req.Status
			} else if strings.Contains(req.Status, "WAITING") {
				statusFormatado = "⏳ " + req.Status
			}

			fmt.Printf("  %-28s %-5d %-8d %-18s %-19s\n", req.ID, req.Prioridade, req.Lamport, statusFormatado, instante)
		}
	}

	fmt.Println("\n======================================================")
	fmt.Println("⌨️  Pressione [ENTER] para acionar o MODO DE COMANDO...")
	fmt.Println("======================================================")
}
