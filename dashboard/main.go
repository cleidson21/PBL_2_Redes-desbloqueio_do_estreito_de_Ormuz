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

// STRUCTS DE DADOS
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
	// ESTADO DO DASHBOARD
	mu          sync.RWMutex
	frota       = make(map[string]EstadoDrone)
	alertas     = make([]string, 0)
	telemetrias = make([]string, 0)

	connMu     sync.RWMutex
	brokerConn net.Conn

	// CONTROLE DE INTERFACE TUI
	uiMu        sync.RWMutex
	modoComando bool // Se true, pausa a atualização automática para o usuário digitar
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
			// Silenciado no modo automático para não poluir a TUI
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

// INTERFACE COM O UTILIZADOR (TUI - Terminal User Interface)
func main() {
	addrVars := os.Getenv("SERVER_ADDRS")
	if addrVars == "" {
		addrVars = "localhost:8083"
	}

	// 1. Inicia a rede em background
	go manterConexaoComBroker(addrVars)

	// 2. Inicia o renderizador automático de tela
	go renderizadorAutomatico()

	// 3. Thread principal bloqueada esperando ações do teclado
	reader := bufio.NewReader(os.Stdin)

	for {
		// Fica aguardando o usuário apertar ENTER para entrar no modo comando
		reader.ReadString('\n')

		// Pausa a atualização automática
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
			// Apenas volta para o loop automático
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

		// Libera a tela para voltar a atualizar automaticamente
		uiMu.Lock()
		modoComando = false
		uiMu.Unlock()
	}
}

// renderizadorAutomatico atualiza a tela a cada segundo se não estiver em modo comando
func renderizadorAutomatico() {
	for {
		uiMu.RLock()
		isComando := modoComando
		uiMu.RUnlock()

		if !isComando {
			limparTela()
			imprimirPainel()
		}
		time.Sleep(1 * time.Second)
	}
}

// limparTela usa códigos ANSI para limpar o terminal e voltar o cursor pro topo
func limparTela() {
	fmt.Print("\033[H\033[2J")
}

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
		mu.Unlock()
	}
}

func imprimirPainel() {
	mu.RLock()
	defer mu.RUnlock()

	// Verifica status da conexão
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

	fmt.Println("\n📡 === LEITURAS DE TELEMETRIA ===")
	if len(telemetrias) == 0 {
		fmt.Println("  Nenhuma telemetria recente.")
	} else {
		for i := len(telemetrias) - 1; i >= 0; i-- {
			fmt.Printf("  📊 %s\n", telemetrias[i])
		}
	}

	fmt.Println("\n======================================================")
	fmt.Println("⌨️  Pressione [ENTER] para acionar o MODO DE COMANDO...")
	fmt.Println("======================================================")
}
