package main

import (
	"fmt"
	"math/rand"
	"net"
	"os"
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
	addrEnv := os.Getenv("SERVER_ADDR")
	if addrEnv == "" {
		addrEnv = "localhost:8081"
	}

	sensorID := os.Getenv("SENSOR_ID")
	if sensorID == "" {
		sensorID = "CATRACA_ENTRADA"
	}

	sensorTipo := os.Getenv("SENSOR_TIPO")
	if sensorTipo == "" {
		sensorTipo = "NFC"
	}

	crachas := []string{"USER_4091", "USER_1192", "USER_5583", "USER_9944"}

	for {
		conn, err := net.Dial("tcp", addrEnv)
		if err != nil {
			fmt.Printf("⚠️ Integrador TCP offline. Tentando novamente em 5 segundos... (%v)\n", err)
			time.Sleep(5 * time.Second)
			continue
		}
		habilitarKeepAlive(conn)

		fmt.Printf("🪪 Sensor [%s] tipo [%s] iniciado! Enviando leituras para %s via TCP.\n", sensorID, sensorTipo, addrEnv)

		for {
			// Sorteia uma leitura e monta o payload esperado pelo integrador.
			crachaLido := crachas[rand.Intn(len(crachas))]

			mensagem := fmt.Sprintf("%s|%s|%s", sensorTipo, sensorID, crachaLido)

			fmt.Printf("Enviando leitura -> %s\n", mensagem)

			// TCP reutiliza a conexao aberta; se falhar, interrompe para reconectar.
			if _, err := fmt.Fprintf(conn, "%s\n", mensagem); err != nil {
				fmt.Printf("⚠️ Falha ao enviar leitura: %v\n", err)
				break
			}

			// Intervalo aleatorio entre leituras para simular fluxo irregular de pessoas.
			tempoEspera := time.Duration(rand.Intn(10)+5) * time.Second
			time.Sleep(tempoEspera)
		}

		conn.Close()
		fmt.Println("❌ Conexão perdida. Iniciando reconexão...")
		time.Sleep(3 * time.Second)
	}
}
