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
		addrEnv = "localhost:8081" // Porta de Eventos Críticos no Broker
	}

	sensorID := os.Getenv("SENSOR_ID")
	if sensorID == "" {
		sensorID = "RADAR_01"
	}

	sensorTipo := strings.ToUpper(os.Getenv("SENSOR_TIPO"))
	if sensorTipo == "" {
		sensorTipo = "RADAR"
	}

	// LÓGICA CAMALEÃO: Define os alertas possíveis com base no tipo do sensor
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
		conn, err := net.Dial("tcp", addrEnv)
		if err != nil {
			fmt.Printf("⚠️ Broker de Setor offline. A tentar novamente em 5 segundos... (%v)\n", err)
			time.Sleep(5 * time.Second)
			continue
		}
		habilitarKeepAlive(conn)

		fmt.Printf("🚨 Sensor Crítico [%s] (Tipo: %s) Iniciado! A enviar eventos para %s via TCP.\n", sensorID, sensorTipo, addrEnv)

		for {
			// Sorteia um evento crítico da lista do "Camaleão"
			eventoSorteado := eventosPossiveis[rand.Intn(len(eventosPossiveis))]

			// Simula uma coordenada GPS próxima ao Estreito de Ormuz (Lat ~26.0, Lon ~56.0)
			lat := 26.0 + (rand.Float64() * 1.5)
			lon := 56.0 + (rand.Float64() * 1.5)
			coordenadaSimulada := fmt.Sprintf("%.4f,%.4f", lat, lon)

			// Montagem do pacote JSON enxuto
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

			// TCP reutiliza a ligação aberta; se falhar, interrompe o for interno para reconectar.
			if _, err := fmt.Fprintf(conn, "%s\n", payload); err != nil {
				fmt.Printf("⚠️ Falha ao enviar o alerta crítico: %v\n", err)
				break
			}

			// Intervalo aleatório ALTO entre leituras (Emergências não acontecem a toda a hora)
			// Simula uma emergência a cada 15 a 45 segundos
			tempoEspera := time.Duration(rand.Intn(30)+15) * time.Second
			time.Sleep(tempoEspera)
		}

		conn.Close()
		fmt.Println("❌ Ligação perdida. A iniciar reconexão...")
		time.Sleep(3 * time.Second)
	}
}
