package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Gottbrok/assinador/nativo/internal/origem"
)

// Os manifestos levam as MESMAS extensões que o programa aceita, e o release sem os IDs das lojas
// não gera manifesto (seria um manifesto que não deixa ninguém entrar).
func TestManifestosComAsExtensoesDoPrograma(t *testing.T) {
	saida := t.TempDir()
	err := gerar(saida, "/usr/lib/confidata-assinador/assinador")
	if len(origem.ExtensoesChrome()) == 0 {
		if err == nil {
			t.Fatal("gerou manifesto sem extensão nenhuma do Chrome")
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	var chromium manifestoChromium
	var firefox manifestoFirefox
	for nome, destino := range map[string]any{"chromium.json": &chromium, "firefox.json": &firefox} {
		b, err := os.ReadFile(filepath.Join(saida, nome))
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, destino); err != nil {
			t.Fatal(err)
		}
	}
	if chromium.Name != "br.com.confidata.assinador" || chromium.Type != "stdio" || chromium.Path != "/usr/lib/confidata-assinador/assinador" {
		t.Fatalf("%+v", chromium)
	}
	for i, id := range origem.ExtensoesChrome() {
		if chromium.AllowedOrigins[i] != "chrome-extension://"+id+"/" {
			t.Fatalf("allowed_origins: %v", chromium.AllowedOrigins)
		}
		chamador, ok := origem.LerChamador([]string{chromium.AllowedOrigins[i]})
		if !ok || !chamador.Permitido() {
			t.Fatalf("o programa recusa a origem que o manifesto autoriza: %s", chromium.AllowedOrigins[i])
		}
	}
	if len(firefox.AllowedExtensions) != 1 || firefox.AllowedExtensions[0] != "assinador@confidata.com.br" || firefox.Name != chromium.Name {
		t.Fatalf("%+v", firefox)
	}
	if err := gerar(saida, "relativo/assinador"); err == nil {
		t.Fatal("aceitou caminho relativo")
	}
}
