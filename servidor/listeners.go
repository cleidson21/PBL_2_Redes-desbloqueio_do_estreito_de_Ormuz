package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

// HabilitarKeepAlive ativa TCP keep-alive para reduzir quedas silenciosas de conexão.
func HabilitarKeepAlive(conn net.Conn) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	_ = tcpConn.SetKeepAlive(true)
	_ = tcpConn.SetKeepAlivePeriod(3 * time.Second)
}

// EnriquecerIdentidade normaliza IDs externos com o namespace local para evitar colisões entre módulos.
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

// AtualizarDashboards replica um evento para todos os dashboards conectados sem bloquear a produção principal.
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

func registrarEstadoDrone(gs *GlobalState, droneID string, estado EstadoDrone) {
	if estado.SeenAt == 0 {
		estado.SeenAt = time.Now().UnixNano()
	}
	gs.FrotaGlobal[droneID] = estado
}

// ListenSensoresTLM consome telemetria UDP, aplica histerese por sensor e enfileira alertas de clima.
func ListenSensoresTLM(gs *GlobalState) {
	addr, _ := net.ResolveUDPAddr("udp", ":48080")
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Printf("❌ [%s] Erro ao iniciar porta UDP 48080: %v\n", gs.MeuNamespace, err)
		return
	}
	defer conn.Close()

	type HisteriseEstado struct {
		estadoVentoAlto bool
		ultimoAlertaID  string
	}
	estadosHisterese := make(map[string]HisteriseEstado)
	limiteSuperior := 70.0
	limiteInferior := 50.0

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

		valorAtualVento, errParse := strconv.ParseFloat(msg.Valor, 64)
		if errParse == nil {
			estado, existe := estadosHisterese[msg.Remetente]
			if !existe {
				estado = HisteriseEstado{estadoVentoAlto: false, ultimoAlertaID: ""}
			}

			if !estado.estadoVentoAlto && valorAtualVento > limiteSuperior {
				estado.estadoVentoAlto = true
				estado.ultimoAlertaID = fmt.Sprintf("%d", time.Now().UnixNano())
				estadosHisterese[msg.Remetente] = estado

				posicao := msg.Posicao
				if posicao == "" {
					posicao = "26.5,56.5"
				}
				// A histerese evita alertas repetidos enquanto o vento permanece acima do limiar.
				fmt.Printf("  🌪️  [ALERTA CLIMÁTICO] Vento forte detetado (%.2f km/h) em %s (sensor: %s). Acionando patrulha drone!\n", valorAtualVento, posicao, msg.Remetente)
				if !gs.AlertQueue.EnqueueAlert(gs, posicao, 1, "") {
					fmt.Printf("⚠️ [ALERTA CLIMÁTICO] Falha ao enfileirar alerta para %s\n", posicao)
				}

			} else if estado.estadoVentoAlto && valorAtualVento < limiteInferior {
				estado.estadoVentoAlto = false
				estadosHisterese[msg.Remetente] = estado

				posicao := msg.Posicao
				if posicao == "" {
					posicao = "26.5,56.5"
				}
				fmt.Printf("  ✅ [CLIMA NORMALIZADO] O vento acalmou em %s (%.2f km/h, sensor: %s).\n", posicao, valorAtualVento, msg.Remetente)
			}
		}

		AtualizarDashboards(gs, msg)
	}
}

// ListenRadarTCP consome eventos críticos TCP, sincroniza Lamport e distribui alertas para o restante do sistema.
func ListenRadarTCP(gs *GlobalState) {
	listener, err := net.Listen("tcp", ":48081")
	if err != nil {
		fmt.Printf("❌ [%s] Erro ao abrir porta TCP 48081: %v\n", gs.MeuNamespace, err)
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
					AtualizarDashboards(gs, msg)
					if !gs.AlertQueue.EnqueueAlert(gs, msg.Posicao, 2, "") {
						fmt.Printf("⚠️ [%s] Alerta crítico rejeitado por fila cheia: %s\n", msg.Remetente, msg.Posicao)
					}
				}
			}
		}(conn)
	}
}

// ListenDrones mantém o estado da frota sincronizado com registros e ACKs dos drones.
func ListenDrones(gs *GlobalState) {
	listener, err := net.Listen("tcp", ":48082")
	if err != nil {
		fmt.Printf("❌ [%s] Erro ao abrir porta TCP 48082: %v\n", gs.MeuNamespace, err)
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
					if estado, existe := gs.FrotaGlobal[droneID]; existe && estado.Status == "DESCONECTADO" {
						fmt.Printf("♻️ [%s] Drone RECONECTADO! Restaurando status para LIVRE.\n", droneID)
						estado.Status = "LIVRE"
						estado.SeenAt = time.Now().UnixNano()
						registrarEstadoDrone(gs, droneID, estado)
					} else if !existe {
						registrarEstadoDrone(gs, droneID, EstadoDrone{Status: "LIVRE", Setor: gs.MeuSetor, SeenAt: time.Now().UnixNano()})
					}
					gs.FrotaMu.Unlock()

					fmt.Printf("🚁 [%s] Drone registado na base local!\n", droneID)

				} else if msg.Tipo == "ACK" {
					gs.FrotaMu.Lock()
					if estado, existe := gs.FrotaGlobal[msg.Remetente]; existe {
						statusAnterior := estado.Status
						estado.Status = msg.Valor
						estado.SeenAt = time.Now().UnixNano()
						registrarEstadoDrone(gs, msg.Remetente, estado)
						if statusAnterior != msg.Valor {
							fmt.Printf("🔄 [%s] Status atualizado: %s → %s\n", msg.Remetente, statusAnterior, msg.Valor)
						}
						gs.FrotaMu.Unlock()
						AtualizarDashboards(gs, msg)
					} else {
						gs.FrotaMu.Unlock()
					}
				}
			}

			if droneID != "" {
				gs.DronesMu.Lock()
				delete(gs.DronesLocais, droneID)
				gs.DronesMu.Unlock()

				gs.FrotaMu.Lock()
				if estado, existe := gs.FrotaGlobal[droneID]; existe {
					estado.Status = "DESCONECTADO"
					estado.SeenAt = time.Now().UnixNano()
					registrarEstadoDrone(gs, droneID, estado)
					fmt.Printf("❌ [%s] Drone desconectado marcado como DESCONECTADO na frota.\n", droneID)
				}
				gs.FrotaMu.Unlock()
			}
			c.Close()
		}(conn)
	}
}

// ListenDashboardTCP recebe comandos do operador e mantém a conexão viva enquanto o dashboard estiver ativo.
func ListenDashboardTCP(gs *GlobalState) {
	listener, err := net.Listen("tcp", ":48083")
	if err != nil {
		fmt.Printf("❌ [%s] Erro ao abrir porta TCP 48083: %v\n", gs.MeuNamespace, err)
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
					if !gs.AlertQueue.EnqueueAlert(gs, msg.Posicao, 1, "") {
						fmt.Printf("⚠️ [%s] Alerta manual rejeitado por fila cheia: %s\n", msg.Remetente, msg.Posicao)
					}
				}
			}
		}(conn)
	}
}

// LimparFrotaExpirada remove drones sem atualização recente para evitar registros fantasmas após failover.
func LimparFrotaExpirada(gs *GlobalState, ttl time.Duration) {
	limite := time.Now().Add(-ttl).UnixNano()

	gs.FrotaMu.Lock()
	defer gs.FrotaMu.Unlock()

	for idDrone, estadoDrone := range gs.FrotaGlobal {
		if estadoDrone.SeenAt == 0 {
			continue
		}
		if estadoDrone.SeenAt < limite {
			delete(gs.FrotaGlobal, idDrone)
			fmt.Printf("🧹 [%s] Drone expirado removido da frota: %s\n", gs.MeuNamespace, idDrone)
		}
	}
}
