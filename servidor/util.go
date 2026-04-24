package main

import (
	"strings"
)

// parseAddressList converte "addr1,addr2,..." em lista de endereços
func parseAddressList(addressStr string) []string {
	var result []string
	parts := strings.Split(addressStr, ",")
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
