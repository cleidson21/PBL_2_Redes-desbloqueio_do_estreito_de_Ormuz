package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
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

func habilitarKeepAlive(conn net.Conn) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	_ = tcpConn.SetKeepAlive(true)
	_ = tcpConn.SetKeepAlivePeriod(3 * time.Second)
}

type EstadoSala struct {
	TemSensorTemp     bool
	TemperaturaAtual  float64
	TemperaturaAlvo   float64
	UltimaLeituraTemp time.Time

	TemAC    bool
	ArLigado bool
	ModoAuto bool

	TemLampada    bool
	LampadaLigada bool

	TemCatraca           bool
	UltimoEvento         string
	UltimaLeituraCatraca time.Time
}

var (
	mu    sync.RWMutex
	salas = make(map[string]*EstadoSala)

	connMu         sync.RWMutex
	integradorConn net.Conn
)

func getSalaSegura(id string) *EstadoSala {
	if _, existe := salas[id]; !existe {
		salas[id] = &EstadoSala{
			TemperaturaAtual: 0.0,
			TemperaturaAlvo:  24.0,
			ArLigado:         false,
			LampadaLigada:    false,
			ModoAuto:         false,
			UltimoEvento:     "Nenhum",
		}
	}
	return salas[id]
}

func setConexao(conn net.Conn) {
	connMu.Lock()
	integradorConn = conn
	connMu.Unlock()
}

func getConexao() net.Conn {
	connMu.RLock()
	defer connMu.RUnlock()
	return integradorConn
}

func descartarConexao(conn net.Conn) {
	connMu.Lock()
	if integradorConn == conn {
		integradorConn = nil
	}
	connMu.Unlock()
	if conn != nil {
		conn.Close()
	}
}

func enviarMensagem(msg Mensagem) bool {
	conn := getConexao()
	if conn == nil {
		return false
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return false
	}

	_, err = fmt.Fprintf(conn, "%s\n", payload)
	if err != nil {
		descartarConexao(conn)
		return false
	}

	return true
}

func manterConexaoComIntegrador(addr string) {
	for {
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			fmt.Printf("⚠️ Integrador offline. Tentando reconectar em 5 segundos... (%v)\n", err)
			time.Sleep(5 * time.Second)
			continue
		}
		habilitarKeepAlive(conn)

		setConexao(conn)
		fmt.Println("✅ Ligado com sucesso ao Gateway Integrador!")

		ouvirRedeEProcessarLogica(conn)

		descartarConexao(conn)
		fmt.Println("❌ Conexão perdida. Iniciando reconexão...")
		time.Sleep(3 * time.Second)
	}
}

// FUNÇÃO PRINCIPAL E INTERFACE COM O UTILIZADOR (CLI)
func main() {
	addrEnv := os.Getenv("INTEGRADOR_ADDR")
	if addrEnv == "" {
		addrEnv = "localhost:8083"
	}

	go manterConexaoComIntegrador(addrEnv)
	go limparDispositivosInativos()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("\n===================================")
		fmt.Println("❄️  PAINEL MULTI-SALA IoT ❄️")
		fmt.Println("===================================")
		fmt.Println("[1] Ver Status de Todas as Salas")
		fmt.Println("[2] Ligar/Desligar Ar (Manual)")
		fmt.Println("[3] Ligar/Desligar Modo Automático")
		fmt.Println("[4] Definir Nova Temperatura Alvo")
		fmt.Println("[5] Ligar/Desligar Lâmpada (Manual)")
		fmt.Println("[0] Sair")
		fmt.Println("===================================")
		fmt.Print("Escolha uma opção: ")

		opcao, _ := reader.ReadString('\n')
		opcao = strings.TrimSpace(opcao)

		switch opcao {
		case "1":
			imprimirPainel()
		case "2":
			fmt.Print("Digite o NOME DA SALA (ex: SALA_1): ")
			idSala, _ := reader.ReadString('\n')
			idSala = strings.TrimSpace(idSala)

			fmt.Print("Digite a AÇÃO (LIGAR ou DESLIGAR): ")
			acao, _ := reader.ReadString('\n')
			acao = strings.TrimSpace(strings.ToUpper(acao))

			mu.Lock()
			sala := getSalaSegura(idSala)
			sala.ModoAuto = false
			mu.Unlock()

			conn := getConexao()
			if conn == nil {
				log.Println("⚠️ FALHA NA REDE: O Gateway caiu! Reiniciando cliente...")
				break
			}

			errSync := enviarMensagem(Mensagem{Tipo: "SYNC", Sala: idSala, Modo: "MANUAL"})
			errCmd := enviarMensagem(Mensagem{Tipo: "CMD", Destino: fmt.Sprintf("AC_%s", idSala), Comando: acao})

			if !errSync || !errCmd {
				log.Println("⚠️ FALHA NA REDE: O Gateway caiu! Reiniciando cliente...")
				descartarConexao(conn)
				break
			}
			fmt.Println("⏳ Comando enviado! (Sincronizando modo MANUAL com a rede...)")

		case "3":
			fmt.Print("Digite o NOME DA SALA (ex: SALA_1): ")
			idSala, _ := reader.ReadString('\n')
			idSala = strings.TrimSpace(idSala)

			mu.Lock()
			sala := getSalaSegura(idSala)
			sala.ModoAuto = !sala.ModoAuto
			statusAuto := sala.ModoAuto
			mu.Unlock()

			conn := getConexao()
			if conn == nil {
				log.Println("⚠️ FALHA NA REDE: O Gateway caiu! Reiniciando cliente...")
				break
			}

			var modo string
			if statusAuto {
				modo = "AUTO"
			} else {
				modo = "MANUAL"
			}

			if !enviarMensagem(Mensagem{Tipo: "SYNC", Sala: idSala, Modo: modo}) {
				log.Println("⚠️ FALHA NA REDE: O Gateway caiu! Reiniciando cliente...")
				descartarConexao(conn)
				break
			}

			if statusAuto {
				fmt.Println("✅ Modo Automático ATIVADO para a sala", idSala)
			} else {
				fmt.Println("🛑 Modo Automático DESATIVADO para a sala", idSala)
			}

		case "4":
			fmt.Print("Digite o NOME DA SALA (ex: SALA_1): ")
			idSala, _ := reader.ReadString('\n')
			idSala = strings.TrimSpace(idSala)

			fmt.Print("Digite a nova TEMPERATURA ALVO (ex: 22.5): ")
			tempStr, _ := reader.ReadString('\n')
			tempVal, err := strconv.ParseFloat(strings.TrimSpace(tempStr), 64)

			if err == nil {
				mu.Lock()
				sala := getSalaSegura(idSala)
				sala.TemperaturaAlvo = tempVal
				mu.Unlock()

				conn := getConexao()
				if conn == nil {
					log.Println("⚠️ FALHA NA REDE: O Gateway caiu! Reiniciando cliente...")
					break
				}

				if !enviarMensagem(Mensagem{Tipo: "CMD", Destino: fmt.Sprintf("AC_%s", idSala), Comando: fmt.Sprintf("SET_TEMP %.1f", tempVal)}) {
					log.Println("⚠️ FALHA NA REDE: O Gateway caiu! Reiniciando cliente...")
					descartarConexao(conn)
					break
				}
				fmt.Printf("🎯 Alvo da %s alterado para %.1f°C\n", idSala, tempVal)
			} else {
				fmt.Println("❌ Valor de temperatura inválido.")
			}

		case "5":
			fmt.Print("Digite o NOME DA SALA (ex: SALA_1): ")
			idSala, _ := reader.ReadString('\n')
			idSala = strings.TrimSpace(idSala)

			fmt.Print("Digite a AÇÃO (LIGAR ou DESLIGAR): ")
			acao, _ := reader.ReadString('\n')
			acao = strings.TrimSpace(strings.ToUpper(acao))

			conn := getConexao()
			if conn == nil {
				log.Println("⚠️ FALHA NA REDE: O Gateway caiu! Reiniciando cliente...")
				break
			}

			if !enviarMensagem(Mensagem{Tipo: "CMD", Destino: fmt.Sprintf("LED_%s", idSala), Comando: acao}) {
				log.Println("⚠️ FALHA NA REDE: O Gateway caiu! Reiniciando cliente...")
				descartarConexao(conn)
				break
			}
			fmt.Println("💡 Comando enviado para a Lâmpada!")

		case "0":
			fmt.Println("A desligar do sistema...")
			return
		default:
			fmt.Println("⚠️ Opção inválida.")
		}
	}
}

// Processa mensagens vindas do integrador e atualiza o estado local.
func ouvirRedeEProcessarLogica(conn net.Conn) {
	scanner := bufio.NewScanner(conn)

	for scanner.Scan() {
		mensagemRaw := strings.TrimSpace(scanner.Text())
		var msg Mensagem
		if err := json.Unmarshal([]byte(mensagemRaw), &msg); err != nil {
			continue
		}
		msg.Tipo = strings.ToUpper(strings.TrimSpace(msg.Tipo))
		if msg.Tipo == "" {
			continue
		}

		mu.Lock()

		if msg.Tipo == "TLM" && msg.Dispositivo == "T" {
			idSala := msg.Sala
			tempAtual, _ := strconv.ParseFloat(msg.Valor, 64)

			sala := getSalaSegura(idSala)
			sala.TemSensorTemp = true
			sala.TemperaturaAtual = tempAtual
			sala.UltimaLeituraTemp = time.Now()

			avaliarModoAutomatico(idSala, sala)
		}

		if msg.Tipo == "EVT" {
			idSala := msg.Sala
			evento := msg.Evento

			sala := getSalaSegura(idSala)
			sala.TemCatraca = true
			sala.UltimoEvento = evento
			sala.UltimaLeituraCatraca = time.Now()
		}

		if msg.Tipo == "ACK" {
			tipoAtuador := strings.ToUpper(msg.Dispositivo)
			idSala := msg.Sala
			acao := msg.Valor

			sala := getSalaSegura(idSala)

			if tipoAtuador == "AC" {
				sala.TemAC = true
				sala.ArLigado = (acao == "LIGADO")
			} else if tipoAtuador == "LED" {
				sala.TemLampada = true
				sala.LampadaLigada = (acao == "LIGADO")
			}
		}

		if msg.Tipo == "SYNC" {
			idSala := msg.Sala
			modo := strings.ToUpper(msg.Modo)

			sala := getSalaSegura(idSala)
			sala.ModoAuto = (modo == "AUTO")
		}

		if msg.Tipo == "ERRO" {
			origem := msg.Origem
			detalhe := msg.Detalhe

			fmt.Printf("\n❌ [FALHA DE COMANDO - %s] %s\n", origem, detalhe)

			if strings.HasPrefix(detalhe, "Atuador ") {
				pedacos := strings.Split(detalhe, " ")
				if len(pedacos) >= 2 {
					idAtuador := pedacos[1]
					idSala := strings.TrimPrefix(idAtuador, "AC_")
					idSala = strings.TrimPrefix(idSala, "LED_")

					sala := getSalaSegura(idSala)

					if strings.HasPrefix(idAtuador, "AC_") {
						sala.TemAC = false
					} else if strings.HasPrefix(idAtuador, "LED_") {
						sala.TemLampada = false
					}

					if sala.ModoAuto {
						sala.ModoAuto = false
						enviarMensagem(Mensagem{Tipo: "SYNC", Sala: idSala, Modo: "MANUAL"})
						fmt.Printf("🛑 MODO AUTOMÁTICO da [%s] foi DESATIVADO na rede por segurança.\n", idSala)
					}
				}
			}

			fmt.Print("Escolha uma opção: ")
		}

		mu.Unlock()
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("⚠️ Leitura da conexão com Integrador falhou: %v\n", err)
	}
}

func limparDispositivosInativos() {
	for {
		time.Sleep(3 * time.Second)

		mu.Lock()
		for id, sala := range salas {
			if sala.TemSensorTemp && time.Since(sala.UltimaLeituraTemp) > 5*time.Second {
				sala.TemSensorTemp = false
			}

			if sala.TemCatraca && time.Since(sala.UltimaLeituraCatraca) > 30*time.Second {
				sala.TemCatraca = false
			}

			if !sala.TemSensorTemp && !sala.TemCatraca && !sala.TemAC && !sala.TemLampada {
				delete(salas, id)
			}
		}
		mu.Unlock()
	}
}

func avaliarModoAutomatico(id string, sala *EstadoSala) {
	if !sala.ModoAuto {
		return
	}

	limiteSuperior := sala.TemperaturaAlvo + 1.0
	limiteInferior := sala.TemperaturaAlvo - 1.0

	if sala.TemperaturaAtual >= limiteSuperior && !sala.ArLigado {
		enviarMensagem(Mensagem{Tipo: "CMD", Destino: fmt.Sprintf("AC_%s", id), Comando: "LIGAR"})
	}

	if sala.TemperaturaAtual <= limiteInferior && sala.ArLigado {
		enviarMensagem(Mensagem{Tipo: "CMD", Destino: fmt.Sprintf("AC_%s", id), Comando: "DESLIGAR"})
	}
}

// Imprime o estado atual das salas e apenas os dispositivos detectados.
func imprimirPainel() {
	mu.RLock()
	defer mu.RUnlock()

	fmt.Println("\n📊 === STATUS ATUAL DA REDE ===")
	if len(salas) == 0 {
		fmt.Println("Nenhum dado recebido ainda. Aguarde pelos sensores ou envie comandos...")
		return
	}

	for id, sala := range salas {
		var blocos []string

		if sala.TemSensorTemp {
			blocos = append(blocos, fmt.Sprintf("🌡️ Temp: %.1f°C", sala.TemperaturaAtual))
			blocos = append(blocos, fmt.Sprintf("🎯 Alvo: %.1f°C", sala.TemperaturaAlvo))

			modo := "✋ MANUAL"
			if sala.ModoAuto {
				modo = "🤖 AUTO"
			}
			blocos = append(blocos, fmt.Sprintf("⚙️ Modo: %s", modo))
		}

		if sala.TemAC {
			statusAr := "🔴 DESLIGADO"
			if sala.ArLigado {
				statusAr = "🟢 LIGADO"
			}
			blocos = append(blocos, fmt.Sprintf("❄️ Ar: %s", statusAr))
		}

		if sala.TemLampada {
			statusLampada := "🌑 APAGADA"
			if sala.LampadaLigada {
				statusLampada = "💡 ACESA"
			}
			blocos = append(blocos, fmt.Sprintf("💡 Lâmpada: %s", statusLampada))
		}

		if sala.TemCatraca {
			blocos = append(blocos, fmt.Sprintf("🪪 Acesso: %s", sala.UltimoEvento))
		}

		// Se ainda nao houver blocos, a sala ainda aguarda deteccao ou confirmacao.
		if len(blocos) == 0 {
			blocos = append(blocos, "⏳ A aguardar identificação dos dispositivos...")
		}

		fmt.Printf("📍 [%s]\n   ↳ %s\n\n", id, strings.Join(blocos, " | "))
	}
	fmt.Println("===============================")
}
