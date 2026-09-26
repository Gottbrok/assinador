package main

import (
	"bytes"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func postar(t *testing.T, h http.Handler, caminho string, corpo any) (int, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(corpo)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, caminho, bytes.NewReader(b)))
	var r map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &r)
	return w.Code, r
}

// O bilhete sai para a origem de localhost, com o documento pedido; forma errada é 400.
func TestServirEmiteBilhete(t *testing.T) {
	chave, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	h := rotasDaPagina(chave, "http://localhost:8787", t.TempDir())
	ref, dig := strings.Repeat("a", 64), strings.Repeat("b", 64)

	codigo, r := postar(t, h, "/bilhete", map[string]string{"ref": ref, "digest": dig, "documento": "Contrato"})
	if codigo != http.StatusOK {
		t.Fatalf("%d %v", codigo, r)
	}
	partes := strings.Split(r["bilhete"].(string), ".")
	carga, _ := base64.RawURLEncoding.DecodeString(partes[1])
	var c map[string]any
	_ = json.Unmarshal(carga, &c)
	if c["aud"] != "http://localhost:8787" || c["dig"] != dig || c["cer"] != ref || c["doc"] != "Contrato" || c["iss"] != "confidata" {
		t.Fatalf("carga: %v", c)
	}

	for _, corpo := range []map[string]string{
		{"ref": "curto", "digest": dig},
		{"ref": ref, "digest": strings.ToUpper(dig)},
		{"ref": ref, "digest": dig, "documento": "a\u0007b"},
		{"ref": ref, "digest": dig, "documento": strings.Repeat("x", 201)},
		{"ref": ref, "digest": dig, "extra": "x"},
	} {
		if codigo, _ := postar(t, h, "/bilhete", corpo); codigo != http.StatusBadRequest {
			t.Errorf("%v: %d", corpo, codigo)
		}
	}
}

// A conferência aceita a assinatura certa e recusa a de outro resumo.
func TestServirConfereAssinatura(t *testing.T) {
	rsaChave, _ := rsa.GenerateKey(rand.Reader, 2048)
	modelo := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "TITULAR:00000000000"}, NotBefore: time.Now(), NotAfter: time.Now().Add(time.Hour)}
	der, _ := x509.CreateCertificate(rand.Reader, modelo, modelo, &rsaChave.PublicKey, rsaChave)
	resumo := sha256.Sum256([]byte("documento"))
	assinatura, _ := rsa.SignPKCS1v15(rand.Reader, rsaChave, crypto.SHA256, resumo[:])

	chave, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	h := rotasDaPagina(chave, "http://localhost:8787", t.TempDir())
	_, r := postar(t, h, "/conferir", map[string]string{"der": base64.StdEncoding.EncodeToString(der), "digest": hex.EncodeToString(resumo[:]), "assinatura": base64.StdEncoding.EncodeToString(assinatura)})
	if r["confere"] != true {
		t.Fatalf("recusou a assinatura certa: %v", r)
	}
	outro := sha256.Sum256([]byte("outro"))
	_, r = postar(t, h, "/conferir", map[string]string{"der": base64.StdEncoding.EncodeToString(der), "digest": hex.EncodeToString(outro[:]), "assinatura": base64.StdEncoding.EncodeToString(assinatura)})
	if r["confere"] != false {
		t.Fatalf("aceitou a assinatura de outro resumo: %v", r)
	}
}

// A página é servida da pasta, e nada fora dela.
func TestServirServeAPagina(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<p>pagina</p>"), 0o600); err != nil {
		t.Fatal(err)
	}
	chave, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	h := rotasDaPagina(chave, "http://localhost:8787", dir)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "pagina") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("%d %q %v", w.Code, w.Body.String(), w.Header())
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/../servir.go", nil))
	if w.Code == http.StatusOK && strings.Contains(w.Body.String(), "package main") {
		t.Fatal("serviu arquivo fora da pasta")
	}
}
