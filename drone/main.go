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

// Mensagem representa o contrato JSON trocado com o servidor.
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

// EstadoDrone descreve o estado operacional reportado para cada drone.
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

	addrVars := os.Getenv("SERVER_ADDRS")
	if addrVars == "" {
		addrVars = os.Getenv("SERVER_ADDR")
	}
	if addrVars == "" {
		addrVars = "localhost:48082"
	}

	listaServidores := strings.Split(addrVars, ",")
	idxServidor := 0

	for {
		addr := strings.TrimSpace(listaServidores[idxServidor])

		conn, err := net.Dial("tcp", addr)
		if err != nil {
			fmt.Printf("⚠️ Falha ao ligar ao servidor %s. A tentar o próximo em 3s... (%v)\n", addr, err)
			idxServidor = (idxServidor + 1) % len(listaServidores)
			time.Sleep(3 * time.Second)
			continue
		}
		habilitarKeepAlive(conn)

		fmt.Printf("✅ [%s] Conectado com sucesso ao servidor em %s\n", droneID, addr)

		if err := enviarMensagem(conn, Mensagem{
			Tipo:      "REG",
			Remetente: droneID,
			Valor:     "DRONE",
		}); err != nil {
			fmt.Printf("⚠️ Falha ao registar o drone: %v\n", err)
			conn.Close()
			idxServidor = (idxServidor + 1) % len(listaServidores)
			time.Sleep(3 * time.Second)
			continue
		}

		done := make(chan bool)

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
						Valor:     estadoEnvio,
						Posicao:   localEnvio,
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
			if err != nil || strings.ToUpper(msg.Tipo) != "CMD" {
				continue
			}

			acao := strings.ToUpper(strings.TrimSpace(msg.Acao))

			switch acao {
			case "DESPACHAR":
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

				// O servidor precisa observar a troca de estado imediatamente para evitar dupla alocação.
				_ = enviarMensagem(conn, Mensagem{
					Tipo:      "ACK",
					Remetente: droneID,
					Valor:     "EM_MISSAO",
					Posicao:   destino,
				})

				// A simulação ocorre em paralelo para manter o processamento de comandos responsivo.
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
			fmt.Printf("⚠️ Ligação com o servidor interrompida: %v\n", err)
		}
		conn.Close()
		fmt.Println("❌ Ligação perdida. Alternando para o próximo servidor de contingência...")
		idxServidor = (idxServidor + 1) % len(listaServidores)
		time.Sleep(3 * time.Second)
	}
}

func simularMissao(conn net.Conn, droneID string, destino string) {
	// Mantém o drone em missão por uma janela fixa para representar a duração operacional do deslocamento.
	time.Sleep(20 * time.Second)

	mu.Lock()
	estadoAtual = "LIVRE"
	localAtual = "BASE"
	mu.Unlock()

	fmt.Printf("✅ [%s] Missão em %s concluída! Retornou à base e está LIVRE.\n", droneID, destino)

	// O ACK final libera o drone para novas atribuições no servidor.
	_ = enviarMensagem(conn, Mensagem{
		Tipo:      "ACK",
		Remetente: droneID,
		Valor:     "LIVRE",
		Posicao:   "BASE",
	})
}
