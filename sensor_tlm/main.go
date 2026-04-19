package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
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
	addrVars := os.Getenv("SERVER_ADDRS")
	if addrVars == "" {
		addrVars = os.Getenv("SERVER_ADDR")
	}
	if addrVars == "" {
		addrVars = "localhost:8080"
	}
	listaServidores := strings.Split(addrVars, ",")
	idxServidor := 0

	var conn *net.UDPConn
	addrAtual := ""

	// Agora o ID reflete o cenário marítimo (ex: BOIA_NORTE_01, RADAR_VENTO)
	sensorID := os.Getenv("SENSOR_ID")
	if sensorID == "" {
		sensorID = "BOIA_01"
	}

	conectarUDP := func(addr string) error {
		if conn != nil {
			conn.Close()
		}
		servidorAddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			return err
		}
		novaConn, err := net.DialUDP("udp", nil, servidorAddr)
		if err != nil {
			return err
		}
		conn = novaConn
		return nil
	}

	for {
		addrAtual = strings.TrimSpace(listaServidores[idxServidor])
		if err := conectarUDP(addrAtual); err != nil {
			fmt.Printf("⚠️ Falha ao ligar ao servidor UDP %s. A tentar o próximo em 3s... (%v)\n", addrAtual, err)
			idxServidor = (idxServidor + 1) % len(listaServidores)
			time.Sleep(3 * time.Second)
			continue
		}
		break
	}
	defer conn.Close()

	fmt.Printf("📡 Sensor de Telemetria [%s] iniciado! Enviando dados para %s via UDP.\n", sensorID, addrAtual)

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
		if conn == nil {
			idxServidor = (idxServidor + 1) % len(listaServidores)
			addrAtual = strings.TrimSpace(listaServidores[idxServidor])
			if err := conectarUDP(addrAtual); err != nil {
				fmt.Printf("⚠️ Falha ao reconectar no servidor UDP %s: %v\n", addrAtual, err)
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}

		_, err := conn.Write(payload)
		if err != nil {
			fmt.Printf("⚠️ Erro de envio para %s: %v. Alternando servidor de contingência...\n", addrAtual, err)

			idxServidor = (idxServidor + 1) % len(listaServidores)
			addrAtual = strings.TrimSpace(listaServidores[idxServidor])
			if errCon := conectarUDP(addrAtual); errCon != nil {
				fmt.Printf("⚠️ Falha ao reconectar no servidor UDP %s: %v\n", addrAtual, errCon)
			}
		}

		// Intervalo fixo entre amostras para manter a taxa de envio.
		time.Sleep(500 * time.Millisecond)
	}
}
