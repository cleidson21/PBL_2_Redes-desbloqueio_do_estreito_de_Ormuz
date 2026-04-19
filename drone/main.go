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
	Setor  string `json:"setor"`
}

var (
	mu          sync.Mutex
	estadoAtual = "LIVRE"
	localAtual  = "BASE"
)

func enviarMensagem(conn net.Conn, msg Mensagem) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(conn, "%s\n", payload)
	return err
}

// Substitui a antiga 'extrairComando'. Agora lidamos direto com a struct!
func lerMensagem(raw string) (Mensagem, error) {
	var msg Mensagem
	err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &msg)
	return msg, err
}

func habilitarKeepAlive(conn net.Conn) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	_ = tcpConn.SetKeepAlive(true)
	_ = tcpConn.SetKeepAlivePeriod(3 * time.Second)
}

func main() {
	droneID := os.Getenv("DRONE_ID")
	if droneID == "" {
		droneID = "DRONE_01"
	}

	integradorAddr := os.Getenv("INTEGRADOR_ADDR")
	if integradorAddr == "" {
		integradorAddr = "localhost:8082" // Porta do Broker do Setor
	}

	for {
		conn, err := net.Dial("tcp", integradorAddr)
		if err != nil {
			fmt.Printf("⚠️ Broker do setor offline. A tentar novamente em 5 segundos... (%v)\n", err)
			time.Sleep(5 * time.Second)
			continue
		}
		habilitarKeepAlive(conn)

		fmt.Printf("🚁 [%s] Iniciado! Ligado ao Broker em %s\n", droneID, integradorAddr)

		// Regista o drone no Broker
		if err := enviarMensagem(conn, Mensagem{
			Tipo:      "REG",
			Remetente: droneID,
			Valor:     "DRONE", // Identifica que este cliente TCP é um Drone
		}); err != nil {
			fmt.Printf("⚠️ Falha ao registar o drone: %v\n", err)
			conn.Close()
			time.Sleep(3 * time.Second)
			continue
		}

		done := make(chan bool)

		// Goroutine do Heartbeat (Sinal de vida e Estado)
		go func() {
			for {
				select {
				case <-time.After(10 * time.Second):
					mu.Lock()
					estadoEnvio := estadoAtual
					localEnvio := localAtual
					mu.Unlock()

					_ = enviarMensagem(conn, Mensagem{
						Tipo:      "ACK",
						Remetente: droneID,
						Valor:     estadoEnvio, // LIVRE, EM_MISSAO...
						Posicao:   localEnvio,  // BASE ou coordenadas
					})
				case <-done:
					log.Printf("Heartbeat parado para %s\n", droneID)
					return
				}
			}
		}()

		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			raw := scanner.Text()
			if strings.TrimSpace(raw) == "" {
				continue
			}

			msg, err := lerMensagem(raw)
			// Só processa se for um JSON válido e do tipo Comando (CMD)
			if err != nil || strings.ToUpper(msg.Tipo) != "CMD" {
				continue
			}

			acao := strings.ToUpper(strings.TrimSpace(msg.Acao))

			switch acao {
			case "DESPACHAR":
				// O JSON já traz a coordenada limpa no campo Posicao!
				destino := msg.Posicao
				if destino == "" {
					destino = "COORDENADAS DESCONHECIDAS"
				}

				mu.Lock()
				if estadoAtual != "LIVRE" {
					fmt.Printf("⚠️ [%s] Comando ignorado: O drone já está %s.\n", droneID, estadoAtual)
					mu.Unlock()
					continue
				}
				estadoAtual = "EM_MISSAO"
				localAtual = destino
				mu.Unlock()

				fmt.Printf("🚀 [%s] DESPACHADO para as coordenadas: %s\n", droneID, destino)

				// Avisa o Broker imediatamente que o estado mudou
				_ = enviarMensagem(conn, Mensagem{
					Tipo:      "ACK",
					Remetente: droneID,
					Valor:     "EM_MISSAO",
					Posicao:   destino,
				})

				// Inicia a simulação da missão numa Goroutine separada para não bloquear a leitura da rede
				go simularMissao(conn, droneID, destino)

			case "RETORNAR":
				mu.Lock()
				estadoAtual = "LIVRE"
				localAtual = "BASE"
				mu.Unlock()

				fmt.Printf("🛬 [%s] Regresso à base confirmado. Estado: LIVRE.\n", droneID)
				_ = enviarMensagem(conn, Mensagem{
					Tipo:      "ACK",
					Remetente: droneID,
					Valor:     "LIVRE",
					Posicao:   "BASE",
				})

			default:
				log.Printf("Comando desconhecido para o Drone: %s\n", acao)
			}
		}

		done <- true

		if err := scanner.Err(); err != nil {
			fmt.Printf("⚠️ Ligação com o Broker interrompida: %v\n", err)
		}
		conn.Close()
		fmt.Println("❌ Ligação perdida. A iniciar reconexão...")
		time.Sleep(3 * time.Second)
	}
}

// Simula o tempo de voo e a resolução do incidente
func simularMissao(conn net.Conn, droneID string, destino string) {
	// Fica "EM_MISSAO" durante 20 segundos
	time.Sleep(20 * time.Second)

	mu.Lock()
	estadoAtual = "LIVRE"
	localAtual = "BASE"
	mu.Unlock()

	fmt.Printf("✅ [%s] Missão em %s concluída! Retornou à base e está LIVRE.\n", droneID, destino)

	// Avisa a rede que acabou o trabalho e está disponível para a próxima emergência
	_ = enviarMensagem(conn, Mensagem{
		Tipo:      "ACK",
		Remetente: droneID,
		Valor:     "LIVRE",
		Posicao:   "BASE",
	})
}
