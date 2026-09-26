package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// construir compila o programa com as tags pedidas num diretório temporário.
func construir(t *testing.T, tags string) string {
	t.Helper()
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatal("o teste compila o programa e precisa do `go` no PATH")
	}
	saida := filepath.Join(t.TempDir(), "assinador")
	args := []string{"build", "-o", saida}
	if tags != "" {
		args = append(args, "-tags", tags)
	}
	cmd := exec.Command(goBin, append(args, ".")...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build %s: %v\n%s", tags, err, out)
	}
	return saida
}

// A catraca da regra 5 do CLAUDE.md: o binário de RELEASE não leva chave de teste, nem o
// carregador das chaves de desenvolvimento, nem o apoio de teste do SoftHSM. O de desenvolvimento
// leva o carregador (prova de que a busca por bytes enxerga o que procura).
func TestReleaseSemChaveDevNemTeste(t *testing.T) {
	if testing.Short() {
		t.Skip("compila o programa duas vezes")
	}
	bruto, err := os.ReadFile(filepath.Join("..", "..", "..", "protocolo", "fixtures", "bilhete", "chaves.json"))
	if err != nil {
		t.Fatal(err)
	}
	var chaves []struct {
		Kid string `json:"kid"`
		Jwk struct {
			X string `json:"x"`
			Y string `json:"y"`
		} `json:"jwk"`
	}
	if err := json.Unmarshal(bruto, &chaves); err != nil {
		t.Fatal(err)
	}
	proibidos := []string{
		"confidata-assinador/chaves-dev.json",
		"github.com/Gottbrok/assinador/nativo/internal/softhsmteste",
		"build de DESENVOLVIMENTO",
	}
	for _, c := range chaves {
		proibidos = append(proibidos, c.Jwk.X, c.Jwk.Y)
	}

	release, err := os.ReadFile(construir(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range proibidos {
		if bytes.Contains(release, []byte(p)) {
			t.Errorf("o binário de release contém %q", p)
		}
	}
	dev, err := os.ReadFile(construir(t, "dev"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(dev, []byte("confidata-assinador/chaves-dev.json")) {
		t.Fatal("o binário dev não tem o carregador: a busca por bytes não prova nada")
	}
}
