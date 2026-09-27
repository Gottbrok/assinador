package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Gottbrok/assinador/nativo/internal/origem"
)

// Os manifestos levam as MESMAS extensões que o programa aceita, e o release sem os IDs das lojas
// não gera manifesto (seria um manifesto que não deixa ninguém entrar).
func TestManifestosComAsExtensoesDoPrograma(t *testing.T) {
	saida := t.TempDir()
	// O caminho absoluto do programa instalado, na forma do sistema que roda o teste.
	programa := "/usr/lib/confidata-assinador/assinador"
	if runtime.GOOS == "windows" {
		programa = `C:\Users\teste\AppData\Local\ConfidataAssinadorDev\assinador.exe`
	}
	err := gerar(saida, programa, false)
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
	if chromium.Name != "br.com.confidata.assinador" || chromium.Type != "stdio" || chromium.Path != programa {
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
	if err := gerar(saida, "relativo/assinador", false); err == nil {
		t.Fatal("aceitou caminho relativo")
	}
}

// O MSI do Windows (F6b) leva os manifestos ao lado do programa, apontando-o só pelo NOME: a pasta por
// usuário só existe na hora da instalação. Qualquer coisa além do nome de um `.exe` é recusada (o que o
// Windows normaliza, nome de dispositivo, caractere proibido ou de controle), e `-relativo` com caminho
// absoluto também (os dois sistemas).
func TestManifestoRelativoSoComONomeDoArquivo(t *testing.T) {
	// A forma do nome vale nos dois builds (o release, sem os IDs das lojas, recusa antes de gerar).
	for _, ruim := range []string{
		"", ".", "..", "pasta/assinador.exe", `pasta\assinador.exe`, `..\assinador.exe`, "C:assinador.exe",
		`C:\Programas\assinador.exe`, "/usr/lib/confidata-assinador/assinador",
		"assinador.exe ", "assinador.exe.", " assinador.exe", "-assinador.exe", "assinador", "assinador.EXE",
		"CON.exe", "nul.exe", "Com1.exe", "lpt9.exe", "aux.dev.exe",
		"assin*dor.exe", "assin?dor.exe", `assin"dor.exe`, "assin<dor.exe", "assin|dor.exe", "assin\x01dor.exe", "assinador\x00.exe",
	} {
		if caminhoValido(ruim, true) {
			t.Errorf("-relativo aceitou %q", ruim)
		}
	}
	for _, bom := range []string{"assinador.exe", "assinador-dev.exe", "Assinador_2.exe", "console.exe", "nulo.exe", "com10.exe"} {
		if !caminhoValido(bom, true) {
			t.Errorf("-relativo recusou %q", bom)
		}
	}

	saida := t.TempDir()
	err := gerar(saida, "assinador.exe", true)
	if len(origem.ExtensoesChrome()) == 0 {
		if err == nil {
			t.Fatal("gerou manifesto sem extensão nenhuma do Chrome")
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, nome := range []string{"chromium.json", "firefox.json"} {
		b, err := os.ReadFile(filepath.Join(saida, nome))
		if err != nil {
			t.Fatal(err)
		}
		var m struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		if m.Path != "assinador.exe" {
			t.Fatalf("%s: path %q", nome, m.Path)
		}
	}
	// E o gerador recusa o que o nome não admite, sem escrever nada por cima.
	if err := gerar(saida, "nul.exe", true); err == nil {
		t.Fatal("-relativo gerou manifesto para nome de dispositivo")
	}
}
