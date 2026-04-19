# PBL 2 - Redes: Desbloqueio do Estreito de Ormuz

Projeto em Go com comunicação TCP/UDP e containers Docker para simular um setor marítimo com servidor central, painel de comando, drones e sensores de radar e telemetria.

## Visao geral

- `servidor`: recebe telemetria, eventos criticos, comandos do painel e conexoes dos drones.
- `dashboard`: painel do operador com estado da frota, telemetria recente e alertas.
- `drone`: agente executante que recebe comando `DESPACHAR` e atualiza seu estado.
- `radar_tcp`: sensor critico que dispara eventos por TCP.
- `sensor_tlm`: sensor de telemetria que envia leituras por UDP.

O servidor tambem mantém a malha P2P na porta `8084/tcp`. Em implantacao com um unico setor, `PEERS` pode ficar vazio. Em multiplos setores, configure a lista de vizinhos para habilitar o consenso distribuido.

## Protocolo de mensagens

- `REG`: registro de drone ou dashboard.
- `CMD`: comando do operador ou do no P2P para disparar missão.
- `ACK`: confirmacao de estado do drone.
- `EVT`: evento critico gerado pelo radar.
- `TLM`: telemetria recebida por UDP.
- `P2P_REQ`, `P2P_CMD`, `P2P_ACK`, `P2P_HELLO`: malha entre servidores.
- `GOSSIP`: sincronizacao da frota entre setores e dashboards.

## Portas

| Porta | Protocolo | Uso |
| --- | --- | --- |
| `8080` | UDP | Entrada de telemetria |
| `8081` | TCP | Entrada de eventos do radar |
| `8082` | TCP | Entrada de drones |
| `8083` | TCP | Entrada do dashboard |
| `8084` | TCP | Malha P2P entre servidores |

## Estrutura

```text
.
├── docker-compose.yml
├── README.md
├── arquivos_sh/
├── dashboard/
├── drone/
├── radar_tcp/
├── sensor_tlm/
└── servidor/
```

## Como executar

```bash
docker compose up -d --build
```

Para abrir o painel interativo:

```bash
docker attach dashboard_operador
```

## Variaveis de ambiente

- `MEU_SETOR`: nome do setor local do servidor.
- `PEERS`: lista de vizinhos P2P separada por virgula, por exemplo `servidor_b:8084,servidor_c:8084`.
- `SERVER_ADDR`: endereco TCP do servidor usado por `dashboard` e `drone`.
- `SERVER_ADDR`: endereco do servidor usado por `radar_tcp` e `sensor_tlm`.
- `DRONE_ID`: identificador do drone.
- `SENSOR_ID`: identificador do sensor.
- `SENSOR_TIPO`: tipo do radar, por exemplo `RADAR`, `AIS` ou `QUIMICO`.

## Scripts

- `arquivos_sh/run_servidor.sh`: sobe apenas o servidor local.
- `arquivos_sh/stress_sensores.sh`: cria sensores de telemetria e radar em massa.
- `arquivos_sh/stress_atuadores.sh`: cria varios drones para teste de carga.
- `arquivos_sh/stress_clientes.sh`: cria varios dashboards para teste de concorrencia.
- `arquivos_sh/cleanup.sh`: remove containers de stress.

## Fluxo de telemetria

O fluxo de telemetria agora sai do `sensor_tlm`, entra no `servidor` e chega ao `dashboard`. Antes desta correção, a mensagem era aceita pelo servidor, mas descartada sem ser exibida no painel.
