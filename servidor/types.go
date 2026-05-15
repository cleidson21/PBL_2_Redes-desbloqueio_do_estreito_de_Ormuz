package main

import (
	"net"
	"sync"
)

// Mensagem representa o envelope JSON usado entre sensores, drones, dashboards e vizinhos.
type Mensagem struct {
	Tipo       string                 `json:"tipo"`
	Remetente  string                 `json:"remetente,omitempty"`
	Destino    string                 `json:"destino,omitempty"`
	Relogio    int                    `json:"relogio,omitempty"`
	Prioridade int                    `json:"prioridade,omitempty"`
	Acao       string                 `json:"acao,omitempty"`
	Valor      string                 `json:"valor,omitempty"`
	Posicao    string                 `json:"posicao,omitempty"`
	Frota      map[string]EstadoDrone `json:"frota,omitempty"`
}

// EstadoDrone descreve o estado atual conhecido de um drone na frota global.
type EstadoDrone struct {
	Status string `json:"status"`
	Setor  string `json:"setor"`
	SeenAt int64  `json:"seen_at,omitempty"`
}

// Alert representa um item de trabalho da fila com prioridade e controle de starvation.
type Alert struct {
	Coordenada    string
	Prioridade    int
	Timestamp     int64
	ID            string
	Lamport       int
	StarveCounter int
}

// AlertQueue gerencia filas separadas de prioridade com prevenção de starvation.
type AlertQueue struct {
	critical        []Alert
	normal          []Alert
	mu              sync.Mutex
	notEmpty        *sync.Cond
	maxSize         int
	starveThreshold int
	processedCount  int
}

// GlobalState reúne o estado compartilhado do servidor e a infraestrutura de concorrência associada.
type GlobalState struct {
	MeuSetor     string
	MeuNamespace string

	RelogioMu sync.Mutex
	Relogio   int

	RadaresMu    sync.RWMutex
	Radares      map[string]net.Conn
	SensoresMu   sync.RWMutex
	Sensores     map[string]*net.UDPAddr
	DronesMu     sync.RWMutex
	DronesLocais map[string]net.Conn
	DashboardsMu sync.RWMutex
	Dashboards   map[net.Conn]bool

	VizinhosMu sync.RWMutex
	Vizinhos   map[string]net.Conn

	FrotaMu     sync.RWMutex
	FrotaGlobal map[string]EstadoDrone

	RicartMu          sync.Mutex
	EstadoRicart      string
	MeuTempoPedido    int
	MinhaPrioridade   int
	RequisicaoAtualID string
	ContadorAging     int
	AcksRecebidos     int
	FilaDeEspera      []Mensagem
	AlvoAtual         string

	AlertQueue *AlertQueue
}

// NewGlobalState cria o estado global inicializado com as estruturas de fila e mapas vazios.
func NewGlobalState(meuSetor string, maxQueueSize, starveThreshold int) *GlobalState {
	gs := &GlobalState{
		MeuSetor:      meuSetor,
		MeuNamespace:  "ORMUZ/" + meuSetor,
		Relogio:       0,
		Radares:       make(map[string]net.Conn),
		Sensores:      make(map[string]*net.UDPAddr),
		DronesLocais:  make(map[string]net.Conn),
		Dashboards:    make(map[net.Conn]bool),
		Vizinhos:      make(map[string]net.Conn),
		FrotaGlobal:   make(map[string]EstadoDrone),
		EstadoRicart:  "LIVRE",
		ContadorAging: 0,
		AcksRecebidos: 0,
	}

	aq := &AlertQueue{
		critical:        make([]Alert, 0, maxQueueSize),
		normal:          make([]Alert, 0, maxQueueSize),
		maxSize:         maxQueueSize,
		starveThreshold: starveThreshold,
	}
	aq.notEmpty = sync.NewCond(&aq.mu)
	gs.AlertQueue = aq

	return gs
}
