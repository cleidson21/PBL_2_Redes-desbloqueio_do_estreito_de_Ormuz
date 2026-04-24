package main

// TickLamport incrementa e retorna o relógio lógico
func TickLamport(gs *GlobalState) int {
	gs.RelogioMu.Lock()
	defer gs.RelogioMu.Unlock()
	gs.Relogio++
	return gs.Relogio
}

// SyncLamport sincroniza o relógio com um valor recebido
func SyncLamport(gs *GlobalState, relogioRecebido int) {
	gs.RelogioMu.Lock()
	defer gs.RelogioMu.Unlock()
	if relogioRecebido > gs.Relogio {
		gs.Relogio = relogioRecebido
	}
	gs.Relogio++
}
