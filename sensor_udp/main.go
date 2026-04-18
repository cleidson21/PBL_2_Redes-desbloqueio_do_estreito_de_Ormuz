package main

import (
	"fmt"
	"net"
	"os"
	"time"
)

func main() {
	addrEnv := os.Getenv("SERVER_ADDR")
	if addrEnv == "" {
		addrEnv = "localhost:8080"
	}

	sensorID := os.Getenv("SENSOR_ID")
	if sensorID == "" {
		sensorID = "SALA_1"
	}

	sensorTipo := os.Getenv("SENSOR_TIPO")
	if sensorTipo == "" {
		sensorTipo = "T"
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

	fmt.Printf("📡 Sensor [%s] tipo [%s] iniciado! Enviando telemetria para %s via UDP.\n", sensorID, sensorTipo, addrEnv)

	// Simula uma leitura oscilando dentro de um intervalo fixo para gerar telemetria continua.
	temperaturaAtual := 25.0
	variacao := 0.33

	for {
		// Inverte o sentido quando atinge os limites para manter a oscilacao.
		temperaturaAtual += variacao

		if temperaturaAtual >= 40.0 {
			temperaturaAtual = 40.0
			variacao = -0.33
		} else if temperaturaAtual <= 16.0 {
			temperaturaAtual = 16.0
			variacao = 0.33
		}

		// Formato do pacote consumido pelo integrador: TIPO|ID|VALOR.
		mensagem := fmt.Sprintf("%s|%s|%.2f", sensorTipo, sensorID, temperaturaAtual)
		fmt.Printf("Enviando -> %s\n", mensagem)

		// UDP nao confirma entrega; o sensor apenas envia e segue o ciclo.
		_, err := conn.Write([]byte(mensagem))
		if err != nil {
			fmt.Printf("⚠️ Erro de rede: %v\n", err)
		}

		// Intervalo fixo entre amostras para manter a taxa de envio.
		time.Sleep(500 * time.Millisecond)
	}
}
