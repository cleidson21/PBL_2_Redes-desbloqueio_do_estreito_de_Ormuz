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

// Mensagem representa o contrato JSON usado para telemetria e eventos.
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

func main() {
	addrVars := os.Getenv("SERVER_ADDRS")
	if addrVars == "" {
		addrVars = os.Getenv("SERVER_ADDR")
	}
	if addrVars == "" {
		addrVars = "localhost:48080"
	}
	listaServidores := strings.Split(addrVars, ",")
	idxServidor := 0

	var conn *net.UDPConn
	addrAtual := ""

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

	fmt.Printf("📡 Sensor de Telemetria [%s] iniciado! Enviando dados para %s via UDP a cada 2s.\n", sensorID, addrAtual)
	fmt.Printf("   Threshold: valor > 70.0 durante 2 leituras consecutivas gera alerta CRÍTICO\n")

	valorAtual := 20.0
	variacao := 0.3

	valorAnterior := 0.0
	contadorAlto := 0
	const THRESHOLD = 70.0
	const CONTADOR_LIMITE = 2

	// A posição fica estática para modelar um sensor fixo na área monitorada.
	posicaoLat := 26.0 + rand.Float64()*2.0
	posicaoLng := 56.0 + rand.Float64()*2.0

	for {
		valorAtual += variacao

		if valorAtual >= 80.0 {
			valorAtual = 80.0
			variacao = -0.3
		} else if valorAtual <= 15.0 {
			valorAtual = 15.0
			variacao = 0.3
		}

		if valorAtual > THRESHOLD {
			contadorAlto++
			if contadorAlto >= CONTADOR_LIMITE && valorAnterior <= THRESHOLD {
				fmt.Printf("🚨 [THRESHOLD ALERT] Sensor %s detectou condição crítica: %.2f > %.2f (em %d leituras)\n", sensorID, valorAtual, THRESHOLD, contadorAlto)
			}
		} else {
			contadorAlto = 0
		}

		mensagem := Mensagem{
			Tipo:      "TLM",
			Remetente: sensorID,
			Valor:     fmt.Sprintf("%.2f", valorAtual),
			Posicao:   fmt.Sprintf("%.4f,%.4f", posicaoLat, posicaoLng),
		}

		payload, errMarshal := json.Marshal(mensagem)
		if errMarshal != nil {
			fmt.Printf("⚠️ Erro ao serializar telemetria JSON: %v\n", errMarshal)
			time.Sleep(2 * time.Second)
			continue
		}

		fmt.Printf("📤 Enviando JSON -> %s\n", payload)

		if conn == nil {
			idxServidor = (idxServidor + 1) % len(listaServidores)
			addrAtual = strings.TrimSpace(listaServidores[idxServidor])
			if err := conectarUDP(addrAtual); err != nil {
				fmt.Printf("⚠️ Falha ao reconectar no servidor UDP %s: %v\n", addrAtual, err)
			}
			time.Sleep(2 * time.Second)
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

		valorAnterior = valorAtual
		time.Sleep(2 * time.Second)
	}
}
