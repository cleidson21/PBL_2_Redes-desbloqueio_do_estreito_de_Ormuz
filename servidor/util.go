package main

import (
	"strings"
)

// parseAddressList converte uma lista separada por vírgulas em endereços normalizados.
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
