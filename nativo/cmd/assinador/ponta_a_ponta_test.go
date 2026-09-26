//go:build !windows

package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Gottbrok/assinador/nativo/internal/origem"
	"github.com/Gottbrok/assinador/nativo/internal/softhsmteste"
)

// A sessão com o programa, o bilhete dev e a chave que o assina estão em `sessao_test.go`, que a
// ponta a ponta do Windows também usa.

// O programa de desenvolvimento, lançado como o Firefox o lança, lista o certificado do token,
// confere o bilhete assinado pela chave dev local e assina com o PIN; a assinatura confere com o
// certificado. Sem navegador: é o que a extensão (F3) vai fazer.
func TestPontaAPontaComSoftHSM(t *testing.T) {
	if testing.Short() {
		t.Skip("compila o programa")
	}
	tk := softhsmteste.Novo(t, softhsmteste.Opcoes{})
	binario := construir(t, "dev")

	// A configuração local: o módulo SoftHSM e a chave dev (a privada só existe na memória do teste).
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	dir := filepath.Join(cfg, "confidata-assinador")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "modulos"), []byte(tk.Modulo+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	chave := gravarChaveDev(t, dir, "dev-ponta-a-ponta")

	s := iniciarPrograma(t, binario, "/usr/lib/mozilla/native-messaging-hosts/br.com.confidata.assinador.json", origem.ExtensaoFirefox)
	s.chave, s.kid = chave, "dev-ponta-a-ponta"

	if r := s.pedir(t, "ola", nil); r["ok"] != true || r["dados"].(map[string]any)["protocolo"] != float64(1) {
		t.Fatalf("ola: %v", r)
	}
	r := s.pedir(t, "listar", nil)
	if r["ok"] != true {
		t.Fatalf("listar: %v", r)
	}
	ref := hex.EncodeToString(func() []byte { h := sha256.Sum256(tk.Certificados[softhsmteste.Titular].Raw); return h[:] }())
	achou := false
	for _, c := range r["dados"].(map[string]any)["certificados"].([]any) {
		cm := c.(map[string]any)
		if cm["ref"] == ref {
			// O nome do provedor depende de quem achou o SoftHSM: o arquivo de configuração deste
			// teste (`libsofthsm2.so`) ou, onde o pacote do sistema o registra, o p11-kit
			// (`softhsm2`), que vem antes e fica com ele.
			provedor, _ := cm["provedor"].(string)
			achou = cm["exigePin"] == true && strings.HasPrefix(provedor, "pkcs11:")
		}
		if cm["ref"] == hex.EncodeToString(func() []byte { h := sha256.Sum256(tk.Certificados[softhsmteste.AC].Raw); return h[:] }()) {
			t.Fatal("a AC entrou na lista")
		}
	}
	if !achou {
		t.Fatalf("o titular não veio como esperado: %v", r)
	}

	digest := sha256.Sum256([]byte("contrato da ponta a ponta"))
	dig := hex.EncodeToString(digest[:])
	r = s.pedir(t, "conferir", map[string]string{"ref": ref, "digest": dig, "bilhete": s.bilhete(t, dig, ref)})
	d, _ := r["dados"].(map[string]any)
	if r["ok"] != true || d["documento"] != "Contrato da ponta a ponta" || d["certificado"].(map[string]any)["assunto"] != "TITULAR DE TESTE:***********" {
		t.Fatalf("conferir: %v", r)
	}
	if d["certificado"].(map[string]any)["exigePin"] != true {
		t.Fatalf("conferir: o SoftHSM exige PIN pelo C_Login, e a janela precisa saber: %v", r)
	}

	r = s.pedir(t, "assinar", map[string]string{"ref": ref, "digest": dig, "bilhete": s.bilhete(t, dig, ref), "pin": "000000"})
	if codigo(r) != "pin-incorreto" {
		t.Fatalf("PIN errado: %v", r)
	}
	r = s.pedir(t, "assinar", map[string]string{"ref": ref, "digest": dig, "bilhete": s.bilhete(t, dig, ref), "pin": softhsmteste.PIN})
	if r["ok"] != true {
		t.Fatalf("assinar: %v", r)
	}
	assinada, err := base64.StdEncoding.DecodeString(r["dados"].(map[string]any)["assinatura"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if err := rsa.VerifyPKCS1v15(tk.Certificados[softhsmteste.Titular].PublicKey.(*rsa.PublicKey), crypto.SHA256, digest[:], assinada); err != nil {
		t.Fatalf("a assinatura não confere: %v", err)
	}

	// Bilhete de outra chave (a de teste das fixtures não é aceita no arquivo dev, e esta nem está lá).
	outra, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	s.chave = outra
	r = s.pedir(t, "assinar", map[string]string{"ref": ref, "digest": dig, "bilhete": s.bilhete(t, dig, ref), "pin": softhsmteste.PIN})
	if codigo(r) != "bilhete-invalido" {
		t.Fatalf("bilhete forjado: %v", r)
	}

	// A entrada fechada encerra o programa, sem erro.
	s.entra.Close()
	fim := make(chan error, 1)
	go func() { fim <- s.cmd.Wait() }()
	select {
	case err := <-fim:
		if err != nil {
			t.Fatalf("o programa saiu com erro: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("o programa não saiu quando a entrada fechou")
	}
}

// O `assinador diagnostico` no terminal mostra o módulo e o certificado do token, com o nome
// mascarado, e nenhum CPF sai (nem no texto, nem no JSON).
func TestDiagnosticoNoTerminal(t *testing.T) {
	if testing.Short() {
		t.Skip("compila o programa")
	}
	tk := softhsmteste.Novo(t, softhsmteste.Opcoes{})
	binario := construir(t, "")
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	if err := os.MkdirAll(filepath.Join(cfg, "confidata-assinador"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg, "confidata-assinador", "modulos"), []byte(tk.Modulo+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	texto, err := exec.Command(binario, "diagnostico").Output()
	if err != nil {
		t.Fatalf("diagnostico: %v", err)
	}
	comoJSON, err := exec.Command(binario, "diagnostico", "--json").Output()
	if err != nil {
		t.Fatalf("diagnostico --json: %v", err)
	}
	var relatorio struct {
		Certificados []struct {
			Titular  string `json:"titular"`
			Situacao string `json:"situacao"`
		} `json:"certificados"`
	}
	if err := json.Unmarshal(comoJSON, &relatorio); err != nil {
		t.Fatalf("o --json não é JSON: %v", err)
	}
	// O SoftHSM aparece como `libsofthsm2.so` (o arquivo de configuração deste teste) ou como
	// `softhsm2` (o p11-kit, onde o pacote do sistema o registra): vale o que ele leu, e ele conta os
	// 5 certificados do token menos o da AC.
	if !strings.Contains(string(texto), ": carregado, 4 certificado(s) (SoftHSM") || !strings.Contains(string(texto), "Certificado: TITULAR DE TESTE:***********") {
		t.Fatalf("texto:\n%s", texto)
	}
	if len(relatorio.Certificados) != 4 { // os 5 do token, menos o da AC
		t.Fatalf("certificados no relatório: %+v", relatorio.Certificados)
	}
	for _, saida := range [][]byte{texto, comoJSON} {
		if strings.Contains(string(saida), "12345678901") {
			t.Fatalf("o CPF vazou:\n%s", saida)
		}
	}
	if out, err := exec.Command(binario, "diagnostico", "--xml").CombinedOutput(); err == nil || !strings.Contains(string(out), "uso:") {
		t.Fatalf("opção desconhecida aceita: %s", out)
	}
}

// Chamado por extensão que não é nossa, o programa não faz nada além de recusar.
func TestChamadorEstranhoERecusado(t *testing.T) {
	if testing.Short() {
		t.Skip("compila o programa")
	}
	binario := construir(t, "")
	s := iniciarPrograma(t, binario, "chrome-extension://abcdefghijklmnopabcdefghijklmnop/")
	if r := s.pedir(t, "ola", nil); codigo(r) != "origem-recusada" {
		t.Fatalf("ola: %v", r)
	}
	if r := s.pedir(t, "listar", nil); codigo(r) != "origem-recusada" || !strings.Contains(r["erro"].(map[string]any)["detalhe"].(string), "chamador") {
		t.Fatalf("listar: %v", r)
	}
}
