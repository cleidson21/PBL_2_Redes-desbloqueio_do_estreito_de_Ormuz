package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strings"
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

// EstadoDrone descreve o estado agregado reportado para cada drone.
type EstadoDrone struct {
	Status string `json:"status"`
	Setor  string `json:"setor"`
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
	addrVars := os.Getenv("SERVER_ADDRS")
	if addrVars == "" {
		addrVars = os.Getenv("SERVER_ADDR")
	}
	if addrVars == "" {
		addrVars = "localhost:48081"
	}

	listaServidores := strings.Split(addrVars, ",")
	idxServidor := 0

	sensorID := os.Getenv("SENSOR_ID")
	if sensorID == "" {
		sensorID = "RADAR_01"
	}

	sensorTipo := strings.ToUpper(os.Getenv("SENSOR_TIPO"))
	if sensorTipo == "" {
		sensorTipo = "RADAR"
	}

	// A seleção dinâmica de eventos permite reutilizar o mesmo sensor para perfis operacionais distintos.
	var eventosPossiveis []string
	switch sensorTipo {
	case "RADAR":
		eventosPossiveis = []string{"OBJETO_NAO_IDENTIFICADO", "ALERTA_DE_CONGESTIONAMENTO"}
	case "AIS":
		eventosPossiveis = []string{"EMBARCACAO_A_DERIVA", "DESVIO_DE_ROTA_CRITICO"}
	case "QUIMICO":
		eventosPossiveis = []string{"VAZAMENTO_DE_OLEO_DETECTADO", "NIVEL_TOXICO_ELEVADO"}
	default:
		eventosPossiveis = []string{"ANOMALIA_GENERICA_DETECTADA"}
	}

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

		fmt.Printf("✅ Sensor Crítico [%s] (Tipo: %s) conectado ao servidor em %s.\n", sensorID, sensorTipo, addr)

		for {
			eventoSorteado := eventosPossiveis[rand.Intn(len(eventosPossiveis))]

			lat := 26.0 + (rand.Float64() * 1.5)
			lon := 56.0 + (rand.Float64() * 1.5)
			coordenadaSimulada := fmt.Sprintf("%.4f,%.4f", lat, lon)

			mensagem := Mensagem{
				Tipo:      "EVT",
				Remetente: sensorID,
				Acao:      "ALERTA",
				Valor:     eventoSorteado,
				Posicao:   coordenadaSimulada,
			}

			payload, errMarshal := json.Marshal(mensagem)
			if errMarshal != nil {
				fmt.Printf("⚠️ Falha ao serializar evento JSON: %v\n", errMarshal)
				tempoEspera := time.Duration(rand.Intn(10)+5) * time.Second
				time.Sleep(tempoEspera)
				continue
			}

			fmt.Printf("A disparar ALERTA JSON -> %s\n", payload)

			if _, err := fmt.Fprintf(conn, "%s\n", payload); err != nil {
				fmt.Printf("⚠️ Falha ao enviar o alerta crítico: %v\n", err)
				break
			}

			// O intervalo aleatório reduz sincronização artificial entre sensores e imita tráfego intermitente.
			tempoEspera := time.Duration(rand.Intn(30)+15) * time.Second
			time.Sleep(tempoEspera)
		}

		conn.Close()
		fmt.Println("❌ Ligação perdida. Alternando para o próximo servidor de contingência...")
		idxServidor = (idxServidor + 1) % len(listaServidores)
		time.Sleep(3 * time.Second)
	}
}
