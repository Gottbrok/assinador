//go:build !windows

package diagnostico

import (
	"bufio"
	"os"
	"strings"
)

// SistemaOperacional é o nome do sistema para o relatório (o PRETTY_NAME do os-release, no
// Linux), ou vazio.
func SistemaOperacional() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return ""
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		if valor, ok := strings.CutPrefix(s.Text(), "PRETTY_NAME="); ok {
			return strings.Trim(valor, `"'`)
		}
	}
	return ""
}
