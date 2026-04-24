package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"
)

// HabilitarKeepAlive ativa TCP keep-alive
func HabilitarKeepAlive(conn net.Conn) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	_ = tcpConn.SetKeepAlive(true)
	_ = tcpConn.SetKeepAlivePeriod(3 * time.Second)
}

// EnriquecerIdentidade adiciona namespace ao ID de um dispositivo
func EnriquecerIdentidade(gs *GlobalState, id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Sprintf("%s/DESCONHECIDO", gs.MeuNamespace)
	}
	if !strings.HasPrefix(id, "ORMUZ") {
		return fmt.Sprintf("%s/%s", gs.MeuNamespace, id)
	}
	return id
}

// AtualizarDashboards notifica todos os dashboards conectados
func AtualizarDashboards(gs *GlobalState, msg Mensagem) {
	payload, err := json.Marshal(msg)
	if err != nil {
		return
	}

	gs.DashboardsMu.RLock()
	defer gs.DashboardsMu.RUnlock()
	for conn := range gs.Dashboards {
		fmt.Fprintf(conn, "%s\n", payload)
	}
}

// ListenSensoresTLM escuta telemetria via UDP
func ListenSensoresTLM(gs *GlobalState) {
	addr, _ := net.ResolveUDPAddr("udp", ":8080")
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Printf("❌ [%s] Erro ao iniciar porta UDP 8080: %v\n", gs.MeuNamespace, err)
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
		msg.Remetente = EnriquecerIdentidade(gs, msg.Remetente)
		fmt.Printf("📡 TELEMETRIA recebida [%s]: %s\n", msg.Remetente, msg.Valor)
		AtualizarDashboards(gs, msg)
	}
}

// ListenRadarTCP escuta eventos críticos via TCP
func ListenRadarTCP(gs *GlobalState) {
	listener, err := net.Listen("tcp", ":8081")
	if err != nil {
		fmt.Printf("❌ [%s] Erro ao abrir porta TCP 8081: %v\n", gs.MeuNamespace, err)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		HabilitarKeepAlive(conn)

		go func(c net.Conn) {
			defer c.Close()
			scanner := bufio.NewScanner(c)
			for scanner.Scan() {
				var msg Mensagem
				if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
					continue
				}

				msg.Remetente = EnriquecerIdentidade(gs, msg.Remetente)
				SyncLamport(gs, msg.Relogio)

				if msg.Tipo == "EVT" && msg.Acao == "ALERTA" {
					fmt.Printf("🚨 ALERTA CRÍTICO DETETADO [%s]: %s em %s\n", msg.Remetente, msg.Valor, msg.Posicao)
					AtualizarDashboards(gs, msg)
					// Enfileirar como alerta crítico (prioridade 2)
					gs.AlertQueue.EnqueueAlert(msg.Posicao, 2)
				}
			}
		}(conn)
	}
}

// ListenDrones escuta drones via TCP
func ListenDrones(gs *GlobalState) {
	listener, err := net.Listen("tcp", ":8082")
	if err != nil {
		fmt.Printf("❌ [%s] Erro ao abrir porta TCP 8082: %v\n", gs.MeuNamespace, err)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		HabilitarKeepAlive(conn)

		go func(c net.Conn) {
			scanner := bufio.NewScanner(c)
			var droneID string

			for scanner.Scan() {
				var msg Mensagem
				if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
					continue
				}

				msg.Remetente = EnriquecerIdentidade(gs, msg.Remetente)
				SyncLamport(gs, msg.Relogio)

				if msg.Tipo == "REG" {
					droneID = msg.Remetente

					gs.DronesMu.Lock()
					gs.DronesLocais[droneID] = c
					gs.DronesMu.Unlock()

					gs.FrotaMu.Lock()
					gs.FrotaGlobal[droneID] = EstadoDrone{Status: "LIVRE", Setor: gs.MeuSetor}
					gs.FrotaMu.Unlock()

					fmt.Printf("🚁 [%s] Drone registado na base local!\n", droneID)

				} else if msg.Tipo == "ACK" {
					gs.FrotaMu.Lock()
					if estado, existe := gs.FrotaGlobal[msg.Remetente]; existe {
						estado.Status = msg.Valor
						gs.FrotaGlobal[msg.Remetente] = estado
					}
					gs.FrotaMu.Unlock()

					AtualizarDashboards(gs, msg)
				}
			}

			if droneID != "" {
				gs.DronesMu.Lock()
				delete(gs.DronesLocais, droneID)
				gs.DronesMu.Unlock()

				gs.FrotaMu.Lock()
				if estado, existe := gs.FrotaGlobal[droneID]; existe {
					estado.Status = "DESCONECTADO"
					gs.FrotaGlobal[droneID] = estado
				}
				gs.FrotaMu.Unlock()
			}
			c.Close()
		}(conn)
	}
}

// ListenDashboardTCP escuta dashboards via TCP
func ListenDashboardTCP(gs *GlobalState) {
	listener, err := net.Listen("tcp", ":8083")
	if err != nil {
		fmt.Printf("❌ [%s] Erro ao abrir porta TCP 8083: %v\n", gs.MeuNamespace, err)
		return
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		HabilitarKeepAlive(conn)

		gs.DashboardsMu.Lock()
		gs.Dashboards[conn] = true
		gs.DashboardsMu.Unlock()

		go func(c net.Conn) {
			defer func() {
				gs.DashboardsMu.Lock()
				delete(gs.Dashboards, c)
				gs.DashboardsMu.Unlock()
				c.Close()
			}()

			scanner := bufio.NewScanner(c)
			for scanner.Scan() {
				var msg Mensagem
				if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
					continue
				}

				msg.Remetente = EnriquecerIdentidade(gs, msg.Remetente)
				SyncLamport(gs, msg.Relogio)

				if msg.Tipo == "CMD" && msg.Acao == "REQUISICAO_MANUAL" {
					fmt.Printf("👨‍💻 Operador solicitou inspeção manual para: %s\n", msg.Posicao)
					// Enfileirar como alerta normal (prioridade 1)
					gs.AlertQueue.EnqueueAlert(msg.Posicao, 1)
				}
			}
		}(conn)
	}
}
