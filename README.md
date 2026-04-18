# PBL 1 - Redes: A Rota das Coisas

Projeto da disciplina de **Conectividade e Concorrencia** com arquitetura IoT distribuida baseada em **Message Broker**, usando **Go + UDP/TCP + Docker**.

> **Atualizacao da arquitetura:** a implementacao atual separa os atuadores em dois servicos independentes (`atuador_ac` e `atuador_led`), mantendo o Integrador como gateway cego e o Cliente como cerebro da logica de negocio.

## Topicos

- [Visao Geral](#visao-geral)
- [Arquitetura Atual](#arquitetura-atual)
- [Componentes do Sistema](#componentes-do-sistema)
- [Protocolo de Mensagens](#protocolo-de-mensagens)
- [Resiliencia e Tolerancia a Falhas](#resiliencia-e-tolerancia-a-falhas)
- [Mapeamento de Portas](#mapeamento-de-portas)
- [Estrutura do Projeto](#estrutura-do-projeto)
- [Como Executar com Docker Compose](#como-executar-com-docker-compose)
- [Como Executar em Rede Distribuida](#como-executar-em-rede-distribuida)
- [Testes de Stress com Scripts .sh](#testes-de-stress-com-scripts-sh)
- [Interface do Cliente (CLI)](#interface-do-cliente-cli)
- [Comandos Docker Uteis](#comandos-docker-uteis)
- [Fluxo de Desenvolvimento](#fluxo-de-desenvolvimento)

---

## Visao Geral

A solucao separa infraestrutura e regra de negocio:

- **Sensores (`sensor_udp`, `sensor_tcp`)**: publicam dados para o Integrador.
- **Integrador (`integrador`)**: roteia mensagens entre sensores, clientes e atuadores.
- **Atuador de Ar (`atuador_ac`)**: recebe comandos de climatizacao (`LIGAR`, `DESLIGAR`, `SET_TEMP`).
- **Atuador de Lampada (`atuador_led`)**: recebe comandos de iluminacao (`LIGAR`, `DESLIGAR`).
- **Cliente (`cliente`)**: mantem estado por sala, aplica histerese termica, sincroniza estado entre paineis (`SYNC`) e executa limpeza automatica de salas inativas (TTL).

---

## Arquitetura Atual

```mermaid
flowchart TD
        S_UDP["sensor_udp\n(telemetria)"]
        S_TCP["sensor_tcp\n(eventos NFC)"]
        I{"integrador\nMessage Broker"}
        C["cliente\nCLI + logica"]
        A_AC["atuador_ac"]
        A_LED["atuador_led"]

        S_UDP -- "UDP 8080" --> I
        S_TCP -- "TCP 8081" --> I
        I <== "TCP 8083" ==> C
        A_AC -- "TCP 8082" --> I
        A_LED -- "TCP 8082" --> I
```

O Integrador apenas encaminha mensagens. Toda regra de automacao fica no Cliente.

---

## Componentes do Sistema

1. **`sensor_udp`**
- Envia leituras continuamente em UDP.
- Formato enviado: `TIPO|SALA|VALOR` (ex.: `T|SALA_1|25.50`).

2. **`sensor_tcp`**
- Envia eventos via conexao TCP persistente.
- Formato enviado: `NFC|CATRACA_ENTRADA|USER_4091`.

3. **`integrador`**
- Mantem lista de atuadores registrados por chave (`AC_SALA_1`, `LED_SALA_1`, etc.).
- Prefixa dados de sensores para os clientes:
    - `TLM|...` para telemetria
    - `EVT|...` para eventos
- Roteia comandos vindos do cliente no formato `ID_ATUADOR|COMANDO`.

4. **`atuador_ac`**
- Registra-se como `REG|AC|<SALA>`.
- Responde com `ACK|AC|<SALA>|<STATUS>`.

5. **`atuador_led`**
- Registra-se como `REG|LED|<SALA>`.
- Responde com `ACK|LED|<SALA>|<STATUS>`.

6. **`cliente`**
- Gerencia dinamicamente um mapa de salas.
- Controle manual de ar e lampada.
- Controle automatico com histerese (`alvo +/- 1.0`) para o ar-condicionado.
- Sincroniza modo/estado entre multiplos clientes via mensagens `SYNC`.
- Remove salas fantasmas com rotina de garbage collector baseada em TTL (janela de 5 a 30 segundos sem telemetria).

---

## Protocolo de Mensagens

Mensagens relevantes na implementacao atual:

- **Sensor UDP -> Integrador**: `T|SALA_1|25.50`
- **Integrador -> Cliente (telemetria)**: `TLM|T|SALA_1|25.50`
- **Sensor TCP -> Integrador**: `NFC|CATRACA_ENTRADA|USER_4091`
- **Integrador -> Cliente (evento)**: `EVT|NFC|CATRACA_ENTRADA|USER_4091`
- **Atuador -> Integrador (registro)**: `REG|AC|SALA_1` ou `REG|LED|SALA_1`
- **Cliente -> Integrador (comando)**: `AC_SALA_1|LIGAR`, `LED_SALA_1|DESLIGAR`, `AC_SALA_1|SET_TEMP 22.5`
- **Cliente -> Integrador (state sync)**: `SYNC|SALA_1|AUTO`, `SYNC|SALA_1|MANUAL`, `SYNC|SALA_1|ALVO 22.5`
- **Integrador -> Clientes (state sync)**: repasse transparente das mensagens `SYNC|...` para evitar split-brain entre paineis.
- **Atuador -> Integrador -> Cliente (ack)**: `ACK|AC|SALA_1|LIGADO`, `ACK|LED|SALA_1|DESLIGADO`

---

## Resiliencia e Tolerancia a Falhas

As versoes mais recentes dos servicos adicionam mecanismos de robustez para operacao distribuida:

1. **Reconexao automatica (cliente, sensores e atuadores)**
- Servicos TCP mantem laço de reconexao com retry/backoff quando o Integrador fica offline.
- O cliente mantem menu interativo ativo e reconecta em background, sem travar a CLI.

2. **Heartbeat de estado nos atuadores (anti late joiner)**
- `atuador_ac` e `atuador_led` enviam `ACK` periodico para que novos clientes recebam estado mesmo entrando depois.
- Heartbeat usa encerramento explicito por canal para evitar vazamento de goroutines apos reconexao.

3. **KeepAlive TCP (detecao de conexao zumbi)**
- Conexoes TCP habilitam keepalive para reduzir permanencia de half-open connections.

4. **QoS no broadcast do Integrador**
- Broadcast para clientes e assincrono, com `write deadline` de 1 segundo para evitar slow consumer bloquear o broker.
- Telemetria com falha de envio e descartada (drop policy) para preservar throughput.

5. **Validacao de payload**
- Integrador valida mensagens malformadas de sensores antes de repassar.


6. **State Sync entre Clientes (anti split-brain)**
- Em cenarios com multiplos paineis, cada cliente publica mudancas de modo/estado com `SYNC|<SALA>|<ESTADO>`.
- Exemplo real: `SYNC|SALA_1|AUTO`.
- Com esse gossip de estado, todos os paineis convergem para a mesma visao logica da sala.

7. **Garbage Collector de Salas (TTL 5-30s)**
- Cada sala tem carimbo de ultimo heartbeat/telemetria.
- Se a sala ficar sem atualizacao por tempo configurado (entre **5 e 30 segundos**, conforme ambiente), ela e removida do mapa em memoria.
- Isso elimina salas fantasmas e trata desconexao de sensores sem intervencao manual.

---

## Mapeamento de Portas

| Protocolo | Porta | Uso |
| --- | --- | --- |
| UDP | `8080` | Entrada de sensores UDP |
| TCP | `8081` | Entrada de sensores TCP |
| TCP | `8082` | Registro e controle de atuadores |
| TCP | `8083` | Conexao de clientes/painel |

---

## Estrutura do Projeto

```text
.
├── docker-compose.yml
├── README.md
├── integrador/
│   ├── Dockerfile
│   ├── go.mod
│   └── main.go
├── cliente/
│   ├── Dockerfile
│   ├── go.mod
│   └── main.go
├── sensor_udp/
│   ├── Dockerfile
│   ├── go.mod
│   └── main.go
├── sensor_tcp/
│   ├── Dockerfile
│   ├── go.mod
│   └── main.go
├── atuador_ac/
│   ├── Dockerfile
│   ├── go.mod
│   └── main.go
├── atuador_led/
│   ├── Dockerfile
│   ├── go.mod
│   └── main.go
└── arquivos_sh/
    ├── cleanup.sh
    ├── run_integrador.sh
    ├── stress_atuadores.sh
    └── stress_sensores.sh
```

---

## Como Executar com Docker Compose

```bash
git clone https://github.com/cleidson21/PBL_1_Redes-A_Rota_das_Coisas.git
cd PBL_1_Redes-A_Rota_das_Coisas

# sobe todo o ecossistema
docker compose up -d --build
```

Abrir interface do cliente:

```bash
docker attach cliente_dashboard
```

Sair sem derrubar o container: `Ctrl+P` e depois `Ctrl+Q`.

Logs uteis:

```bash
docker logs -f integrador_gateway
docker logs -f sensor_temp_sala1
docker logs -f sensor_nfc_entrada
docker logs -f atuador_ar_sala1
docker logs -f atuador_lampada_sala1
```

---

## Como Executar em Rede Distribuida

Exemplo em 3 maquinas:

- **PC 1 (Gateway)**: Integrador
- **PC 2 (Borda)**: Sensores e atuadores
- **PC 3 (Operacao)**: Cliente

1. **PC 1 - Integrador**

```bash
docker run -d --name integrador_pbl \
    -p 8080:8080/udp -p 8081:8081/tcp -p 8082:8082/tcp -p 8083:8083/tcp \
    cleidsonramos/integrador:v2
```

2. **PC 2 - Dispositivos**

```bash
# Sensor UDP
docker run -d --name sensor_udp_pbl \
    -e SERVER_ADDR="<IP_GATEWAY>:8080" \
    -e SENSOR_ID="SALA_1" \
    -e SENSOR_TIPO="T" \
    cleidsonramos/sensor_udp:v2

# Sensor TCP
docker run -d --name sensor_tcp_pbl \
    -e SERVER_ADDR="<IP_GATEWAY>:8081" \
    -e SENSOR_ID="CATRACA_ENTRADA" \
    -e SENSOR_TIPO="NFC" \
    cleidsonramos/sensor_tcp:v2

# Atuador AC
docker run -d --name atuador_ac_pbl \
    -e INTEGRADOR_ADDR="<IP_GATEWAY>:8082" \
    -e ATUADOR_ID="SALA_1" \
    -e ATUADOR_TIPO="AC" \
    cleidsonramos/atuador_ac:v2

# Atuador LED
docker run -d --name atuador_led_pbl \
    -e INTEGRADOR_ADDR="<IP_GATEWAY>:8082" \
    -e ATUADOR_ID="SALA_1" \
    -e ATUADOR_TIPO="LED" \
    cleidsonramos/atuador_led:v2
```

3. **PC 3 - Cliente**

```bash
docker run -it --name cliente_pbl \
    -e INTEGRADOR_ADDR="<IP_GATEWAY>:8083" \
    cleidsonramos/cliente:v2
```

---

## Testes de Stress com Scripts .sh

O projeto possui scripts para facilitar testes de carga e cenarios distribuidos:

- `run_integrador.sh`: inicia o Integrador com as portas `8080/udp`, `8081/tcp`, `8082/tcp` e `8083/tcp`.
- `stress_sensores.sh`: sobe sensores UDP + TCP em lote.
- `stress_atuadores.sh`: sobe atuadores AC + LED em lote.
- `stress_clientes.sh`: sobe clientes em lote.
- `cleanup.sh`: remove os containers de stress (`stress_*`).

### Sequencia recomendada

```bash
# 1) iniciar gateway
bash run_integrador.sh

# 2) iniciar sensores
bash stress_sensores.sh

# 3) iniciar atuadores
bash stress_atuadores.sh

```

### Cenario minimo (quantidade 1)

Se quiser usar os mesmos scripts para subir um ambiente minimo (1 sala), altere em cada script de stress:

```bash
QTD_SALAS=1
```

Com `QTD_SALAS=1`, os scripts sobem:

- 1 sensor UDP + 1 sensor TCP
- 1 atuador AC + 1 atuador LED
- 1 cliente

Para acompanhar o painel desse cliente, que precisam ser implementados manualmente por conta do menu interativo:

```bash
docker attach stress_cliente_1
```

Para limpar o ambiente de stress:

```bash
bash cleanup.sh
```

---

## Interface do Cliente (CLI)

```text
===================================
PAINEL MULTI-SALA IoT
===================================
[1] Ver Status de Todas as Salas
[2] Ligar/Desligar Ar (Manual)
[3] Ligar/Desligar Modo Automatico
[4] Definir Nova Temperatura Alvo
[5] Ligar/Desligar Lampada (Manual)
[0] Sair
===================================
```

Detalhes importantes:

- Comando manual de ar desativa o modo automatico da sala.
- Ajuste de alvo envia `SET_TEMP` para o atuador AC e atualiza o estado local.
- Novas salas sao criadas automaticamente quando chegam dados com novo `ID`.

---

## Comandos Docker Uteis

```bash
# listar containers
docker ps

# parar e remover stack local
docker compose down

# reconstruir apenas um servico
docker compose up -d --build cliente

# limpeza geral
docker stop $(docker ps -aq) 2>/dev/null; docker system prune -a --volumes -f
```

---

## Fluxo de Desenvolvimento

Rebuild de imagens por servico:

```bash
docker build -t cleidsonramos/integrador:v3 ./integrador
docker build -t cleidsonramos/cliente:v3 ./cliente
docker build -t cleidsonramos/sensor_udp:v3 ./sensor_udp
docker build -t cleidsonramos/sensor_tcp:v3 ./sensor_tcp
docker build -t cleidsonramos/atuador_ac:v3 ./atuador_ac
docker build -t cleidsonramos/atuador_led:v3 ./atuador_led
```

Publicacao (opcional):

```bash
docker push cleidsonramos/integrador:v3
docker push cleidsonramos/cliente:v3
docker push cleidsonramos/sensor_udp:v3
docker push cleidsonramos/sensor_tcp:v3
docker push cleidsonramos/atuador_ac:v3
docker push cleidsonramos/atuador_led:v3
```