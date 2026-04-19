package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"
)

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

func main() {
	addrEnv := os.Getenv("SERVER_ADDR")
	if addrEnv == "" {
		addrEnv = "localhost:8080"
	}

	// Agora o ID reflete o cenário marítimo (ex: BOIA_NORTE_01, RADAR_VENTO)
	sensorID := os.Getenv("SENSOR_ID")
	if sensorID == "" {
		sensorID = "BOIA_01"
	}

	servidorAddr, err := net.ResolveUDPAddr("udp", addrEnv)
	if err != nil {
		fmt.Printf("❌ Erro ao resolver endereço: %v\n", err)
		return
	}

	conn, err := net.DialUDP("udp", nil, servidorAddr)
	if err != nil {
		fmt.Printf("❌ Erro ao conectar: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Printf("📡 Sensor de Telemetria [%s] iniciado! Enviando dados para %s via UDP.\n", sensorID, addrEnv)

	// Simula uma leitura oscilando dentro de um intervalo fixo.
	// Pode representar Vento (km/h), Corrente (m/s), etc.
	valorAtual := 20.0
	variacao := 1.5

	for {
		// Inverte o sentido quando atinge os limites para manter a oscilação.
		valorAtual += variacao

		if valorAtual >= 85.0 {
			valorAtual = 85.0
			variacao = -1.5
		} else if valorAtual <= 10.0 {
			valorAtual = 10.0
			variacao = 1.5
		}

		// Montagem do pacote JSON enxuto
		mensagem := Mensagem{
			Tipo:      "TLM",
			Remetente: sensorID,
			Valor:     fmt.Sprintf("%.2f", valorAtual),
		}

		payload, errMarshal := json.Marshal(mensagem)
		if errMarshal != nil {
			fmt.Printf("⚠️ Erro ao serializar telemetria JSON: %v\n", errMarshal)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		fmt.Printf("Enviando JSON -> %s\n", payload)

		// UDP não confirma entrega; o sensor apenas envia e segue o ciclo.
		_, err := conn.Write(payload)
		if err != nil {
			fmt.Printf("⚠️ Erro de rede: %v\n", err)
		}

		// Intervalo fixo entre amostras para manter a taxa de envio.
		time.Sleep(500 * time.Millisecond)
	}
}
