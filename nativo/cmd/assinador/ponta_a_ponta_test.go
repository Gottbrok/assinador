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
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Gottbrok/assinador/nativo/internal/mensagens"
	"github.com/Gottbrok/assinador/nativo/internal/origem"
	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
	"github.com/Gottbrok/assinador/nativo/internal/softhsmteste"
)

const origemLocal = "http://localhost:3000"

// sessao é o programa de verdade, lançado como o Firefox lança um host de native messaging.
type sessao struct {
	cmd   *exec.Cmd
	entra io.WriteCloser
	sai   io.ReadCloser
	seq   int
	chave *ecdsa.PrivateKey
	kid   string
}

func (s *sessao) pedir(t *testing.T, op string, dados map[string]string) map[string]any {
	t.Helper()
	s.seq++
	m := map[string]any{"v": 1, "id": "e2e-" + string(rune('a'+s.seq)), "op": op, "origem": origemLocal}
	if dados != nil {
		m["dados"] = dados
	}
	b, _ := json.Marshal(m)
	if err := mensagens.EscreverQuadro(s.entra, b, protocolo.EntradaMaxima); err != nil {
		t.Fatal(err)
	}
	resposta, err := mensagens.LerQuadro(s.sai, protocolo.SaidaMaxima)
	if err != nil {
		t.Fatal(err)
	}
	var r map[string]any
	if err := json.Unmarshal(resposta, &r); err != nil {
		t.Fatal(err)
	}
	if r["id"] != m["id"] {
		t.Fatalf("id trocado: %v", r)
	}
	return r
}

// bilhete assina, com a chave dev deste teste, o bilhete que o servidor local emitiria.
func (s *sessao) bilhete(t *testing.T, digest, cer string) string {
	t.Helper()
	agora := time.Now().Unix()
	cab, _ := json.Marshal(map[string]any{"alg": "ES256", "typ": "assinador+jws", "kid": s.kid})
	carga, _ := json.Marshal(map[string]any{"v": 1, "iss": "confidata", "aud": origemLocal, "sid": "e2e", "dig": digest, "cer": cer,
		"fin": "assinatura", "doc": "Contrato da ponta a ponta", "org": "Assinador (teste)", "iat": agora, "exp": agora + 300})
	entrada := base64.RawURLEncoding.EncodeToString(cab) + "." + base64.RawURLEncoding.EncodeToString(carga)
	h := sha256.Sum256([]byte(entrada))
	r, sg, err := ecdsa.Sign(rand.Reader, s.chave, h[:])
	if err != nil {
		t.Fatal(err)
	}
	a := make([]byte, 64)
	r.FillBytes(a[:32])
	sg.FillBytes(a[32:])
	return entrada + "." + base64.RawURLEncoding.EncodeToString(a)
}

func iniciarPrograma(t *testing.T, binario string, args ...string) *sessao {
	t.Helper()
	cmd := exec.Command(binario, args...)
	entra, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	sai, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	return &sessao{cmd: cmd, entra: entra, sai: sai}
}

func codigo(r map[string]any) string {
	if e, ok := r["erro"].(map[string]any); ok {
		return e["codigo"].(string)
	}
	return ""
}

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
	chave, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ponto, _ := chave.PublicKey.Bytes()
	jwk, _ := json.Marshal([]map[string]any{{"kid": "dev-ponta-a-ponta", "iss": "confidata", "ambiente": "dev", "jwk": map[string]string{
		"kty": "EC", "crv": "P-256", "x": base64.RawURLEncoding.EncodeToString(ponto[1:33]), "y": base64.RawURLEncoding.EncodeToString(ponto[33:]),
	}}})
	if err := os.WriteFile(filepath.Join(dir, "chaves-dev.json"), jwk, 0o600); err != nil {
		t.Fatal(err)
	}

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
	// `softhsm2` (o p11-kit, onde o pacote do sistema o registra): vale o que ele leu.
	if !strings.Contains(string(texto), ": carregado, 5 certificado(s) (SoftHSM") || !strings.Contains(string(texto), "Certificado: TITULAR DE TESTE:***********") {
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
