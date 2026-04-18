package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
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

func enviarMensagem(conn net.Conn, msg Mensagem) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(conn, "%s\n", payload)
	return err
}

func extrairComando(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	var msg Mensagem
	if err := json.Unmarshal([]byte(raw), &msg); err == nil {
		if strings.ToUpper(strings.TrimSpace(msg.Tipo)) == "CMD" {
			return strings.TrimSpace(msg.Comando)
		}
	}

	return raw
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
	atuadorID := os.Getenv("ATUADOR_ID")
	if atuadorID == "" {
		atuadorID = "SALA_1"
	}

	tipoAtuador := os.Getenv("ATUADOR_TIPO")
	if tipoAtuador == "" {
		tipoAtuador = "LED"
	}

	integradorAddr := os.Getenv("INTEGRADOR_ADDR")
	if integradorAddr == "" {
		integradorAddr = "localhost:8082"
	}

	for {
		conn, err := net.Dial("tcp", integradorAddr)
		if err != nil {
			fmt.Printf("⚠️ Integrador offline. Tentando novamente em 5 segundos... (%v)\n", err)
			time.Sleep(5 * time.Second)
			continue
		}
		habilitarKeepAlive(conn)

		fmt.Printf("⚙️  [%s] %s Iniciado! Conectado em %s\n", atuadorID, tipoAtuador, integradorAddr)

		if err := enviarMensagem(conn, Mensagem{
			Tipo:        "REG",
			Dispositivo: tipoAtuador,
			Sala:        atuadorID,
		}); err != nil {
			fmt.Printf("⚠️ Falha ao registrar atuador: %v\n", err)
			conn.Close()
			time.Sleep(3 * time.Second)
			continue
		}

		estadoAtual := "DESLIGADO"

		done := make(chan bool)

		go func() {
			for {
				select {
				case <-time.After(10 * time.Second):
					_ = enviarMensagem(conn, Mensagem{
						Tipo:        "ACK",
						Dispositivo: tipoAtuador,
						Sala:        atuadorID,
						Valor:       estadoAtual,
					})
				case <-done:
					log.Printf("Heartbeat stopped for %s\n", atuadorID)
					return
				}
			}
		}()

		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			comando := extrairComando(scanner.Text())
			if comando == "" {
				continue
			}
			partes := strings.Split(comando, " ")
			acao := partes[0]

			switch acao {
			case "LIGAR":
				fmt.Printf("💡 [%s] Lâmpada ACESA...\n", atuadorID)
				estadoAtual = "LIGADO"
				_ = enviarMensagem(conn, Mensagem{
					Tipo:        "ACK",
					Dispositivo: tipoAtuador,
					Sala:        atuadorID,
					Valor:       "LIGADO",
				})
			case "DESLIGAR":
				fmt.Printf("🌑 [%s] Lâmpada APAGADA...\n", atuadorID)
				estadoAtual = "DESLIGADO"
				_ = enviarMensagem(conn, Mensagem{
					Tipo:        "ACK",
					Dispositivo: tipoAtuador,
					Sala:        atuadorID,
					Valor:       "DESLIGADO",
				})
			default:
				log.Printf("Comando desconhecido para Lâmpada: %s\n", comando)
			}
		}

		done <- true

		if err := scanner.Err(); err != nil {
			fmt.Printf("⚠️ Conexão com integrador interrompida: %v\n", err)
		}
		conn.Close()
		fmt.Println("❌ Conexão perdida. Iniciando reconexão...")
		time.Sleep(3 * time.Second)
	}
}
