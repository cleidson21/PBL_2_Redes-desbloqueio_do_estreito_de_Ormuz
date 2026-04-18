package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"
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

		// Registra o atuador no gateway com o formato REG|TIPO|ID.
		fmt.Fprintf(conn, "REG|%s|%s\n", tipoAtuador, atuadorID)

		estadoAtual := "DESLIGADO"

		done := make(chan bool)

		go func() {
			for {
				select {
				case <-time.After(10 * time.Second):
					fmt.Fprintf(conn, "ACK|%s|%s|%s\n", tipoAtuador, atuadorID, estadoAtual)
				case <-done:
					log.Printf("Heartbeat stopped for %s\n", atuadorID)
					return
				}
			}
		}()

		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			comando := strings.TrimSpace(scanner.Text())
			partes := strings.Split(comando, " ")
			acao := partes[0]

			switch acao {
			case "LIGAR":
				fmt.Printf("💡 [%s] Lâmpada ACESA...\n", atuadorID)
				estadoAtual = "LIGADO"
				fmt.Fprintf(conn, "ACK|%s|%s|LIGADO\n", tipoAtuador, atuadorID)
			case "DESLIGAR":
				fmt.Printf("🌑 [%s] Lâmpada APAGADA...\n", atuadorID)
				estadoAtual = "DESLIGADO"
				fmt.Fprintf(conn, "ACK|%s|%s|DESLIGADO\n", tipoAtuador, atuadorID)
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
