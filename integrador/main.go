package main

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

var (
	muAtuadores sync.RWMutex
	atuadores   = make(map[string]net.Conn)

	muClientes sync.RWMutex
	clientes   = make(map[net.Conn]bool)
)

func habilitarKeepAlive(conn net.Conn) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	_ = tcpConn.SetKeepAlive(true)
	_ = tcpConn.SetKeepAlivePeriod(3 * time.Second)
}

func main() {
	fmt.Println("🚀 Integrador Gateway (Broker) Iniciado!")
	fmt.Println("Ouvindo as seguintes portas:")
	fmt.Println("- UDP 8080: Sensores de Temperatura Contínuos")
	fmt.Println("- TCP 8081: Sensores de Eventos (NFC/Catraca)")
	fmt.Println("- TCP 8082: Atuadores (Ar Condicionado/Lâmpadas)")
	fmt.Println("- TCP 8083: Clientes (Dashboards e Controladores)")

	go listenSensoresUDP()
	go listenSensoresTCP()
	go listenAtuadoresTCP()
	go listenClientesTCP()

	select {}
}

// Porta 8080: recebe telemetria UDP de sensores contínuos.
func listenSensoresUDP() {
	addr, _ := net.ResolveUDPAddr("udp", ":8080")
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Printf("❌ Erro ao iniciar servidor UDP 8080: %v\n", err)
		return
	}
	defer conn.Close()

	buffer := make([]byte, 1024)
	for {
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			continue
		}

		mensagem := strings.TrimSpace(string(buffer[:n]))
		partes := strings.Split(mensagem, "|")
		if len(partes) < 3 {
			fmt.Printf("⚠️ [Sensor UDP] Payload inválido descartado: %s\n", mensagem)
			continue
		}

		fmt.Printf("📥 [Sensor UDP] Recebeu: %s\n", mensagem)
		broadcastParaClientes("TLM|" + mensagem)
	}
}

// Porta 8081: recebe eventos TCP de sensores com conexao persistente.
func listenSensoresTCP() {
	listener, err := net.Listen("tcp", ":8081")
	if err != nil {
		fmt.Printf("❌ Erro ao iniciar servidor TCP 8081 (Sensores): %v\n", err)
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
				mensagem := strings.TrimSpace(scanner.Text())
				partes := strings.Split(mensagem, "|")
				if len(partes) < 3 {
					fmt.Printf("⚠️ [Sensor TCP] Payload inválido descartado: %s\n", mensagem)
					continue
				}

				fmt.Printf("📥 [Sensor TCP] Recebeu: %s\n", mensagem)
				broadcastParaClientes("EVT|" + mensagem)
			}
		}(conn)
	}
}

// Porta 8082: registra atuadores e encaminha respostas deles para os clientes.
func listenAtuadoresTCP() {
	listener, err := net.Listen("tcp", ":8082")
	if err != nil {
		fmt.Printf("❌ Erro ao iniciar servidor TCP 8082 (Atuadores): %v\n", err)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		habilitarKeepAlive(conn)
		go manipularAtuador(conn)
	}
}

func manipularAtuador(conn net.Conn) {
	scanner := bufio.NewScanner(conn)
	var atuadorID string

	for scanner.Scan() {
		mensagem := strings.TrimSpace(scanner.Text())
		partes := strings.Split(mensagem, "|")
		if len(partes) == 0 || partes[0] == "" {
			continue
		}

		// Se for uma mensagem de REGISTRO (Ex: REG|AC|SALA_1)
		if partes[0] == "REG" && len(partes) >= 3 {
			tipoAtuador := partes[1]
			idSala := partes[2]

			atuadorID = fmt.Sprintf("%s_%s", tipoAtuador, idSala)

			muAtuadores.Lock()
			atuadores[atuadorID] = conn
			muAtuadores.Unlock()

			fmt.Printf("⚙️  [Atuador] %s registrado com sucesso!\n", atuadorID)
			continue
		}

		if (partes[0] == "ACK" || partes[0] == "ERRO") && len(partes) >= 3 {
			fmt.Printf("📤 [Atuador -> Cliente] Repassando: %s\n", mensagem)
			broadcastParaClientes(mensagem)
		}
	}

	// Se o código chegar aqui, a conexão caiu. Removemos do Dicionário.
	if atuadorID != "" {
		muAtuadores.Lock()
		delete(atuadores, atuadorID)
		muAtuadores.Unlock()
		fmt.Printf("⚠️ [Atuador] %s desconectado.\n", atuadorID)
	}
	conn.Close()
}

// Porta 8083: recebe clientes e dashboards de controle.
func listenClientesTCP() {
	listener, err := net.Listen("tcp", ":8083")
	if err != nil {
		fmt.Printf("❌ Erro ao iniciar servidor TCP 8083 (Clientes): %v\n", err)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		habilitarKeepAlive(conn)

		// Mantem a conexao do cliente registrada para broadcast.
		muClientes.Lock()
		clientes[conn] = true
		muClientes.Unlock()

		fmt.Println("📱 [Cliente] Novo Dashboard conectado!")
		go manipularCliente(conn)
	}
}

func manipularCliente(conn net.Conn) {
	scanner := bufio.NewScanner(conn)

	// Comandos chegam no formato ID_ATUADOR|COMANDO ou SYNC|COMANDO.
	for scanner.Scan() {
		mensagem := strings.TrimSpace(scanner.Text())
		partes := strings.SplitN(mensagem, "|", 2) // Corta apenas no primeiro "|"

		if len(partes) == 2 {
			idDestino := partes[0]
			comando := partes[1]

			// Canal de sincronizacao entre clientes para replicar estado.
			if idDestino == "SYNC" {
				fmt.Printf("🔄 [State Sync] Espalhando sincronização: %s\n", comando)
				broadcastParaClientes(fmt.Sprintf("SYNC|%s", comando))
				continue
			}

			// Busca a conexao do atuador pelo ID logico.
			muAtuadores.RLock()
			atuadorConn, existe := atuadores[idDestino]
			muAtuadores.RUnlock()

			if existe {
				fmt.Printf("📤 [Cliente -> Atuador %s] Roteando comando: %s\n", idDestino, comando)
				fmt.Fprintf(atuadorConn, "%s\n", comando)
			} else {
				fmt.Printf("⚠️ [Cliente] Atuador %s offline\n", idDestino)
				fmt.Fprintf(conn, "ERRO|GATEWAY|Atuador %s offline\n", idDestino)
			}
		}
	}

	// Remove o cliente da lista ao encerrar a conexao.
	muClientes.Lock()
	delete(clientes, conn)
	muClientes.Unlock()
	fmt.Println("📱 [Cliente] Dashboard desconectado.")
	conn.Close()
}

// Broadcast para todos os clientes conectados. (COM QoS GARANTIDO)
func broadcastParaClientes(mensagem string) {
	muClientes.RLock()
	defer muClientes.RUnlock()

	isTelemetria := strings.HasPrefix(mensagem, "TLM|")

	for conn := range clientes {
		go func(clienteConn net.Conn, msg string, telemetria bool) {
			defer clienteConn.SetWriteDeadline(time.Time{})
			clienteConn.SetWriteDeadline(time.Now().Add(1 * time.Second))
			_, err := fmt.Fprintf(clienteConn, "%s\n", msg)
			if err != nil && telemetria {
				return
			}
		}(conn, mensagem, isTelemetria)
	}
}
