//go:build windows

package diagnostico

import "golang.org/x/sys/windows/registry"

// SistemaOperacional é o nome do sistema para o relatório, lido do registro (só leitura), ou
// vazio.
func SistemaOperacional() string {
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	valor := func(nome string) string {
		v, _, err := k.GetStringValue(nome)
		if err != nil {
			return ""
		}
		return v
	}
	return nomeDoWindows(valor("ProductName"), valor("DisplayVersion"), valor("CurrentBuild"))
}
