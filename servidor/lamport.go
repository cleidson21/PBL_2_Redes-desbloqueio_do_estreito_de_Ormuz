package main

// TickLamport avança o relógio lógico local para representar um novo evento.
func TickLamport(gs *GlobalState) int {
	gs.RelogioMu.Lock()
	defer gs.RelogioMu.Unlock()
	gs.Relogio++
	return gs.Relogio
}

// SyncLamport ajusta o relógio local para preservar a ordem causal antes de avançá-lo.
func SyncLamport(gs *GlobalState, relogioRecebido int) {
	gs.RelogioMu.Lock()
	defer gs.RelogioMu.Unlock()
	if relogioRecebido > gs.Relogio {
		gs.Relogio = relogioRecebido
	}
	gs.Relogio++
}
