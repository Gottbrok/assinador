package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Gottbrok/assinador/nativo/internal/mensagens"
	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
)

// O que as pontas a ponta (a do Linux, com o SoftHSM, e a do Windows, com o repositório do usuário)
// compartilham: o programa de verdade numa sessão de native messaging, e o bilhete assinado por uma
// chave dev que só existe na memória do teste.

const origemLocal = "http://localhost:3000"

// sessao é o programa de verdade, lançado como o navegador lança um host de native messaging.
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

// gravarChaveDev cria uma chave dev (a privada só existe na memória do teste) e grava a pública em
// `chaves-dev.json` da pasta de configuração `dir` (a `confidata-assinador` do sistema), onde o
// programa de desenvolvimento a procura.
func gravarChaveDev(t *testing.T, dir, kid string) *ecdsa.PrivateKey {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	chave, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ponto, _ := chave.PublicKey.Bytes()
	jwk, _ := json.Marshal([]map[string]any{{"kid": kid, "iss": "confidata", "ambiente": "dev", "jwk": map[string]string{
		"kty": "EC", "crv": "P-256", "x": base64.RawURLEncoding.EncodeToString(ponto[1:33]), "y": base64.RawURLEncoding.EncodeToString(ponto[33:]),
	}}})
	if err := os.WriteFile(filepath.Join(dir, "chaves-dev.json"), jwk, 0o600); err != nil {
		t.Fatal(err)
	}
	return chave
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
