//go:build linux

package main

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Gottbrok/assinador/nativo/internal/mensagens"
	"github.com/Gottbrok/assinador/nativo/internal/origem"
	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
)

// O host de verdade, com o PC/SC apontado para a biblioteca falsa que ESCREVE na saída padrão ao
// abrir o contexto (como uma biblioteca descuidada faria): a resposta do diagnóstico chega inteira
// pelo canal, e o que a biblioteca escreveu não aparece nele. É a prova de ponta a ponta de
// `separarCanal`, no processo que roda o C.
func TestCanalDoHostNaoRecebeOQueOCEscreve(t *testing.T) {
	if testing.Short() {
		t.Skip("compila o programa")
	}
	gcc, err := exec.LookPath("gcc")
	if err != nil {
		t.Skip("sem gcc")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatal("o teste compila o programa e precisa do `go` no PATH")
	}
	dir := t.TempDir()
	falso := filepath.Join(dir, "libpcsclite-falso.so")
	fonte := filepath.Join("..", "..", "testes", "pcsc-falso", "pcsc.c")
	if out, err := exec.Command(gcc, "-shared", "-fPIC", "-I", filepath.Join("..", "..", "internal", "pcsc"), "-o", falso, fonte).CombinedOutput(); err != nil {
		t.Fatalf("gcc: %v %s", err, out)
	}
	binario := filepath.Join(dir, "assinador")
	ldflags := "-X github.com/Gottbrok/assinador/nativo/internal/pcsc.biblioteca=" + falso
	if out, err := exec.Command(goBin, "build", "-ldflags", ldflags, "-o", binario, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build: %v %s", err, out)
	}

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cmd := exec.Command(binario, "/manifesto.json", origem.ExtensaoFirefox)
	cmd.Env = append(os.Environ(), "ASSINADOR_PCSC_FALSO=escreve")
	entra, _ := cmd.StdinPipe()
	sai, _ := cmd.StdoutPipe()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })

	if err := mensagens.EscreverQuadro(entra, []byte(`{"v":1,"id":"canal","op":"diagnostico","origem":"https://ushield.app"}`), protocolo.EntradaMaxima); err != nil {
		t.Fatal(err)
	}
	corpo, err := mensagens.LerQuadro(sai, protocolo.SaidaMaxima)
	if err != nil {
		t.Fatalf("o quadro não chegou inteiro (o que o C escreveu entrou no canal?): %v", err)
	}
	var resposta struct {
		ID    string `json:"id"`
		OK    bool   `json:"ok"`
		Dados struct {
			Relatorio struct {
				Leitoras []struct {
					Nome string `json:"nome"`
				} `json:"leitoras"`
			} `json:"relatorio"`
		} `json:"dados"`
	}
	if err := json.Unmarshal(corpo, &resposta); err != nil {
		t.Fatalf("resposta que não é JSON: %v %q", err, corpo)
	}
	if resposta.ID != "canal" || !resposta.OK || len(resposta.Dados.Relatorio.Leitoras) != 2 {
		t.Fatalf("a biblioteca falsa não foi usada, ou a resposta veio errada: %s", corpo)
	}
	entra.Close()
	resto, _ := io.ReadAll(sai)
	if len(resto) != 0 || strings.Contains(string(corpo), "lixo") {
		t.Fatalf("o canal recebeu o que a biblioteca escreveu: %q", resto)
	}
}
