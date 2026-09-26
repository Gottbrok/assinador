//go:build extensao && linux

package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Gottbrok/assinador/nativo/internal/origem"
	"github.com/Gottbrok/assinador/nativo/internal/softhsmteste"
)

// TestExtensaoNoNavegador é o ponta a ponta da F3: o Chromium do Playwright carrega a extensão de
// desenvolvimento (`extensao/dist/chrome-dev`, montada antes pelo `npm run ponta-a-ponta`), acha o
// programa de desenvolvimento pelo manifesto do host no diretório de dados DELE, e o programa lê o
// token SoftHSM. A página de teste é o `host-teste servir`, que emite o bilhete com uma chave dev
// gerada aqui e confere a assinatura. Quem dirige o navegador é `extensao/e2e/extensao.spec.ts`.
//
// Fica atrás da tag `extensao` porque precisa do Node, das dependências da extensão e do Chromium
// do Playwright; o `go test` comum não o roda.
func TestExtensaoNoNavegador(t *testing.T) {
	raiz, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	extensao := filepath.Join(raiz, "extensao", "dist", "chrome-dev")
	if _, err := os.Stat(filepath.Join(extensao, "manifest.json")); err != nil {
		t.Fatalf("falta a extensão de desenvolvimento em %s: rode pelo `npm run ponta-a-ponta` (que a monta antes)", extensao)
	}

	tk := softhsmteste.Novo(t, softhsmteste.Opcoes{})
	binario := construir(t, "dev")

	// A configuração do programa (o módulo SoftHSM e a chave pública dev) vive num XDG_CONFIG_HOME só
	// deste teste, que o navegador herda e passa ao programa.
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	if err := os.MkdirAll(filepath.Join(cfg, "confidata-assinador"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "confidata-assinador", "modulos"), []byte(tk.Modulo+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	trabalho := t.TempDir()
	hostTeste := filepath.Join(trabalho, "host-teste")
	executar(t, filepath.Join(raiz, "ferramentas"), "go", "build", "-o", hostTeste, "./host-teste")
	// A chave dev do bilhete: a privada fica no XDG deste teste, a pública vai ao chaves-dev.json.
	executar(t, raiz, hostTeste, "gerar-chave")

	// O manifesto do host sai do gerador (regra 13: nunca à mão), com o ID de desenvolvimento, e vai
	// para o diretório de dados do navegador de teste, onde o Chromium procura os hosts do usuário.
	manifestos := filepath.Join(trabalho, "manifestos")
	executar(t, filepath.Join(raiz, "nativo"), "go", "run", "-tags", "dev", "./cmd/manifestos", "-saida", manifestos, "-programa", binario)
	dados := filepath.Join(trabalho, "navegador")
	destino := filepath.Join(dados, "NativeMessagingHosts")
	if err := os.MkdirAll(destino, 0o700); err != nil {
		t.Fatal(err)
	}
	manifesto, err := os.ReadFile(filepath.Join(manifestos, "chromium.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destino, origem.NomeDoHost+".json"), manifesto, 0o600); err != nil {
		t.Fatal(err)
	}

	url := servirPagina(t, hostTeste, filepath.Join(raiz, "extensao", "e2e", "pagina"))

	ref := sha256.Sum256(tk.Certificados[softhsmteste.Titular].Raw)
	ctx, cancelar := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancelar()
	cmd := exec.CommandContext(ctx, "npx", "playwright", "test", "-c", "e2e/playwright.config.ts")
	cmd.Dir = filepath.Join(raiz, "extensao")
	cmd.Env = append(os.Environ(),
		"ASSINADOR_E2E_URL="+url,
		"ASSINADOR_E2E_DADOS="+dados,
		"ASSINADOR_E2E_EXTENSAO="+extensao,
		"ASSINADOR_E2E_PIN="+softhsmteste.PIN,
		"ASSINADOR_E2E_REF="+hex.EncodeToString(ref[:]),
	)
	saida, err := cmd.CombinedOutput()
	t.Logf("playwright:\n%s", saida)
	if err != nil {
		t.Fatalf("o teste da extensão no navegador falhou: %v", err)
	}
	if strings.Contains(string(saida), "skipped") && !strings.Contains(string(saida), "passed") {
		t.Fatal("o Playwright pulou os testes: as variáveis não chegaram")
	}
}

// executar roda um comando e reprova com a saída dele se falhar.
func executar(t *testing.T, dir, nome string, args ...string) {
	t.Helper()
	cmd := exec.Command(nome, args...)
	cmd.Dir = dir
	if saida, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%s %s: %v\n%s", nome, strings.Join(args, " "), err, saida)
	}
}

// servirPagina sobe o `host-teste servir` numa porta livre e devolve a origem que ele anunciou.
func servirPagina(t *testing.T, hostTeste, pagina string) string {
	t.Helper()
	cmd := exec.Command(hostTeste, "servir", "--porta", "0", "--pagina", pagina)
	saida, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	linha := make(chan string, 1)
	go func() {
		s := bufio.NewScanner(saida)
		if s.Scan() {
			linha <- s.Text()
		}
		close(linha)
	}()
	select {
	case l := <-linha:
		url, ok := strings.CutPrefix(l, "servindo em ")
		if !ok {
			t.Fatalf("o host-teste não anunciou a origem: %q", l)
		}
		return url
	case <-time.After(30 * time.Second):
		t.Fatal("o host-teste não subiu")
	}
	return ""
}
