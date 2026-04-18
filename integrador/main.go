package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

type Mensagem struct {
	Tipo        string `json:"tipo"`
	Origem      string `json:"origem,omitempty"`
	Dispositivo string `json:"dispositivo,omitempty"`
	Sala        string `json:"sala,omitempty"`
	Destino     string `json:"destino,omitempty"`
	Comando     string `json:"comando,omitempty"`
	Evento      string `json:"evento,omitempty"`
	Valor       string `json:"valor,omitempty"`
	Modo        string `json:"modo,omitempty"`
	Detalhe     string `json:"detalhe,omitempty"`
}

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

func parseMensagemJSON(raw string) (Mensagem, bool) {
	var msg Mensagem
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		return Mensagem{}, false
	}

	msg.Tipo = strings.ToUpper(strings.TrimSpace(msg.Tipo))
	if msg.Tipo == "" {
		return Mensagem{}, false
	}

	msg.Dispositivo = strings.TrimSpace(msg.Dispositivo)
	msg.Sala = strings.TrimSpace(msg.Sala)
	msg.Destino = strings.TrimSpace(msg.Destino)
	msg.Comando = strings.TrimSpace(msg.Comando)
	msg.Evento = strings.TrimSpace(msg.Evento)
	msg.Valor = strings.TrimSpace(msg.Valor)
	msg.Modo = strings.ToUpper(strings.TrimSpace(msg.Modo))
	msg.Detalhe = strings.TrimSpace(msg.Detalhe)
	msg.Origem = strings.TrimSpace(msg.Origem)

	return msg, true
}

func parseSensorLegacy(raw string, tipoEsperado string) (Mensagem, bool) {
	partes := strings.Split(raw, "|")
	if len(partes) < 3 {
		return Mensagem{}, false
	}

	tipo := strings.ToUpper(strings.TrimSpace(partes[0]))
	sala := strings.TrimSpace(partes[1])
	valor := strings.TrimSpace(partes[2])

	if tipo == "" || sala == "" || valor == "" {
		return Mensagem{}, false
	}

	msg := Mensagem{
		Tipo:        tipoEsperado,
		Dispositivo: tipo,
		Sala:        sala,
	}

	if tipoEsperado == "TLM" {
		msg.Valor = valor
	} else {
		msg.Evento = valor
	}

	return msg, true
}

func parseAtuadorLegacy(raw string) (Mensagem, bool) {
	partes := strings.Split(raw, "|")
	if len(partes) < 3 {
		return Mensagem{}, false
	}

	tipo := strings.ToUpper(strings.TrimSpace(partes[0]))
	switch tipo {
	case "REG":
		return Mensagem{
			Tipo:        "REG",
			Dispositivo: strings.ToUpper(strings.TrimSpace(partes[1])),
			Sala:        strings.TrimSpace(partes[2]),
		}, true
	case "ACK":
		msg := Mensagem{
			Tipo:        "ACK",
			Dispositivo: strings.ToUpper(strings.TrimSpace(partes[1])),
			Sala:        strings.TrimSpace(partes[2]),
		}
		if len(partes) >= 4 {
			msg.Valor = strings.TrimSpace(strings.Join(partes[3:], "|"))
		}
		return msg, true
	case "ERRO":
		msg := Mensagem{
			Tipo:   "ERRO",
			Origem: strings.TrimSpace(partes[1]),
		}
		if len(partes) >= 3 {
			msg.Detalhe = strings.TrimSpace(strings.Join(partes[2:], "|"))
		}
		return msg, true
	default:
		return Mensagem{}, false
	}
}

func parseComandoCliente(raw string) (Mensagem, bool) {
	if msg, ok := parseMensagemJSON(raw); ok {
		switch msg.Tipo {
		case "SYNC":
			if msg.Sala == "" || msg.Modo == "" {
				return Mensagem{}, false
			}
			return msg, true
		case "CMD":
			if msg.Destino == "" || msg.Comando == "" {
				return Mensagem{}, false
			}
			return msg, true
		default:
			return Mensagem{}, false
		}
	}

	partes := strings.SplitN(raw, "|", 2)
	if len(partes) != 2 {
		return Mensagem{}, false
	}

	idDestino := strings.TrimSpace(partes[0])
	corpo := strings.TrimSpace(partes[1])
	if idDestino == "" || corpo == "" {
		return Mensagem{}, false
	}

	if strings.ToUpper(idDestino) == "SYNC" {
		syncPartes := strings.SplitN(corpo, "|", 2)
		if len(syncPartes) != 2 {
			return Mensagem{}, false
		}
		return Mensagem{
			Tipo: "SYNC",
			Sala: strings.TrimSpace(syncPartes[0]),
			Modo: strings.ToUpper(strings.TrimSpace(syncPartes[1])),
		}, true
	}

	return Mensagem{
		Tipo:    "CMD",
		Destino: idDestino,
		Comando: corpo,
	}, true
}

func enviarParaCliente(conn net.Conn, msg Mensagem, telemetria bool) {
	defer conn.SetWriteDeadline(time.Time{})
	conn.SetWriteDeadline(time.Now().Add(1 * time.Second))

	payloadJSON, err := json.Marshal(msg)
	if err != nil {
		return
	}

	if _, err := fmt.Fprintf(conn, "%s\n", payloadJSON); err != nil {
		if telemetria {
			return
		}
	}
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

		mensagemRaw := strings.TrimSpace(string(buffer[:n]))

		msg, ok := parseMensagemJSON(mensagemRaw)
		if ok {
			if msg.Tipo != "TLM" || msg.Dispositivo == "" || msg.Sala == "" || msg.Valor == "" {
				fmt.Printf("⚠️ [Sensor UDP] JSON inválido descartado: %s\n", mensagemRaw)
				continue
			}
		} else {
			msg, ok = parseSensorLegacy(mensagemRaw, "TLM")
			if !ok {
				fmt.Printf("⚠️ [Sensor UDP] Payload inválido descartado: %s\n", mensagemRaw)
				continue
			}
		}

		fmt.Printf("📥 [Sensor UDP] Recebeu: %+v\n", msg)
		broadcastParaClientes(msg)
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
				mensagemRaw := strings.TrimSpace(scanner.Text())

				msg, ok := parseMensagemJSON(mensagemRaw)
				if ok {
					if msg.Tipo != "EVT" || msg.Dispositivo == "" || msg.Sala == "" || msg.Evento == "" {
						fmt.Printf("⚠️ [Sensor TCP] JSON inválido descartado: %s\n", mensagemRaw)
						continue
					}
				} else {
					msg, ok = parseSensorLegacy(mensagemRaw, "EVT")
					if !ok {
						fmt.Printf("⚠️ [Sensor TCP] Payload inválido descartado: %s\n", mensagemRaw)
						continue
					}
				}

				fmt.Printf("📥 [Sensor TCP] Recebeu: %+v\n", msg)
				broadcastParaClientes(msg)
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
		mensagemRaw := strings.TrimSpace(scanner.Text())
		if mensagemRaw == "" {
			continue
		}

		msg, ok := parseMensagemJSON(mensagemRaw)
		if !ok {
			msg, ok = parseAtuadorLegacy(mensagemRaw)
			if !ok {
				fmt.Printf("⚠️ [Atuador] Payload inválido descartado: %s\n", mensagemRaw)
				continue
			}
		}

		if msg.Tipo == "REG" {
			atuadorID = fmt.Sprintf("%s_%s", strings.ToUpper(msg.Dispositivo), msg.Sala)

			muAtuadores.Lock()
			atuadores[atuadorID] = conn
			muAtuadores.Unlock()

			fmt.Printf("⚙️  [Atuador] %s registrado com sucesso!\n", atuadorID)
			continue
		}

		if msg.Tipo == "ACK" || msg.Tipo == "ERRO" {
			fmt.Printf("📤 [Atuador -> Cliente] Repassando: %+v\n", msg)
			broadcastParaClientes(msg)
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

	// Comandos chegam em JSON (preferencial) ou no legado ID_ATUADOR|COMANDO.
	for scanner.Scan() {
		mensagemRaw := strings.TrimSpace(scanner.Text())
		msg, ok := parseComandoCliente(mensagemRaw)
		if !ok {
			fmt.Printf("⚠️ [Cliente] Comando inválido descartado: %s\n", mensagemRaw)
			continue
		}

		if msg.Tipo == "SYNC" {
			fmt.Printf("🔄 [State Sync] Espalhando sincronização JSON: %+v\n", msg)
			broadcastParaClientes(msg)
			continue
		}

		// Busca a conexao do atuador pelo ID logico.
		muAtuadores.RLock()
		atuadorConn, existe := atuadores[msg.Destino]
		muAtuadores.RUnlock()

		if existe {
			fmt.Printf("📤 [Cliente -> Atuador %s] Roteando comando: %s\n", msg.Destino, msg.Comando)
			payloadCmd, err := json.Marshal(Mensagem{Tipo: "CMD", Comando: msg.Comando})
			if err != nil {
				enviarParaCliente(conn, Mensagem{
					Tipo:    "ERRO",
					Origem:  "GATEWAY",
					Detalhe: "Falha ao serializar comando para atuador",
				}, false)
				continue
			}
			fmt.Fprintf(atuadorConn, "%s\n", payloadCmd)
		} else {
			fmt.Printf("⚠️ [Cliente] Atuador %s offline\n", msg.Destino)
			enviarParaCliente(conn, Mensagem{
				Tipo:    "ERRO",
				Origem:  "GATEWAY",
				Detalhe: fmt.Sprintf("Atuador %s offline", msg.Destino),
			}, false)
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
func broadcastParaClientes(msg Mensagem) {
	muClientes.RLock()
	defer muClientes.RUnlock()

	isTelemetria := msg.Tipo == "TLM"

	for conn := range clientes {
		go func(clienteConn net.Conn, payload Mensagem, telemetria bool) {
			enviarParaCliente(clienteConn, payload, telemetria)
		}(conn, msg, isTelemetria)
	}
}
