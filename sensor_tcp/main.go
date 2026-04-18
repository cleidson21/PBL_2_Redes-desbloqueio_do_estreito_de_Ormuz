package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"time"
)

type Mensagem struct {
	Tipo        string `json:"tipo"`
	Dispositivo string `json:"dispositivo"`
	Sala        string `json:"sala"`
	Valor       string `json:"valor,omitempty"`
	Evento      string `json:"evento,omitempty"`
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
			// Sorteia uma leitura e monta o payload JSON esperado pelo integrador.
			crachaLido := crachas[rand.Intn(len(crachas))]

			mensagem := Mensagem{
				Tipo:        "EVT",
				Dispositivo: sensorTipo,
				Sala:        sensorID,
				Evento:      crachaLido,
			}
			payload, errMarshal := json.Marshal(mensagem)
			if errMarshal != nil {
				fmt.Printf("⚠️ Falha ao serializar evento JSON: %v\n", errMarshal)
				tempoEspera := time.Duration(rand.Intn(10)+5) * time.Second
				time.Sleep(tempoEspera)
				continue
			}

			fmt.Printf("Enviando leitura JSON -> %s\n", payload)

			// TCP reutiliza a conexao aberta; se falhar, interrompe para reconectar.
			if _, err := fmt.Fprintf(conn, "%s\n", payload); err != nil {
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
