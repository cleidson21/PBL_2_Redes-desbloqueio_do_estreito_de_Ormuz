package main

import (
	"net"
	"sync"
)

// Mensagem é a estrutura padrão de comunicação
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

type EstadoDrone struct {
	Status string `json:"status"`
	Setor  string `json:"setor"`
	SeenAt int64  `json:"seen_at,omitempty"`
}

// Alert representa um alerta crítico ou normal na fila
type Alert struct {
	Coordenada    string
	Prioridade    int // 1=normal, 2=crítico
	Timestamp     int64
	ID            string
	StarveCounter int // conta quantos ciclos críticos passaram sem atender este alert
}

// AlertQueue gerencia fila de alta e baixa prioridade com starvation prevention
type AlertQueue struct {
	critical        []Alert // fila de alertas críticos (prioridade 2)
	normal          []Alert // fila de alertas normais (prioridade 1)
	mu              sync.Mutex
	notEmpty        *sync.Cond
	maxSize         int
	starveThreshold int // após N ciclos críticos, normal sobe para crítico
	processedCount  int // conta quantos alertas críticos foram processados
}

// GlobalState contém todas as variáveis globais compartilhadas
type GlobalState struct {
	// Identidade do setor
	MeuSetor     string
	MeuNamespace string

	// Lamport clock
	RelogioMu sync.Mutex
	Relogio   int

	// Conexões
	RadaresMu    sync.RWMutex
	Radares      map[string]net.Conn
	SensoresMu   sync.RWMutex
	Sensores     map[string]*net.UDPAddr
	DronesMu     sync.RWMutex
	DronesLocais map[string]net.Conn
	DashboardsMu sync.RWMutex
	Dashboards   map[net.Conn]bool

	// P2P
	VizinhosMu sync.RWMutex
	Vizinhos   map[string]net.Conn

	// Frota global
	FrotaMu     sync.RWMutex
	FrotaGlobal map[string]EstadoDrone

	// Ricart-Agrawala
	RicartMu        sync.Mutex
	EstadoRicart    string // LIVRE, ESPERANDO, USANDO
	MeuTempoPedido  int
	MinhaPrioridade int
	ContadorAging   int
	AcksRecebidos   int
	FilaDeEspera    []Mensagem
	AlvoAtual       string

	// Fila de alertas
	AlertQueue *AlertQueue
}

// NewGlobalState cria uma nova instância do estado global
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

	// Initialize alert queue
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
