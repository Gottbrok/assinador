//go:build windows

package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	win "golang.org/x/sys/windows"

	"github.com/Gottbrok/assinador/nativo/internal/windows"
	"github.com/Gottbrok/assinador/nativo/internal/windowsteste"
)

// O programa de desenvolvimento no Windows, lançado como o Chrome o lança (o ID da extensão e a
// janela-mãe), lista o certificado com chave CNG do repositório do usuário sem pedir PIN, confere o
// bilhete da chave dev local e assina; a assinatura confere com o certificado. Tudo passa pelo canal
// separado da saída padrão (`canal_windows.go`).
func TestPontaAPontaNoWindows(t *testing.T) {
	if testing.Short() {
		t.Skip("compila o programa")
	}
	c := windowsteste.Novo(t, windowsteste.KSP)
	x, err := x509.ParseCertificate(c.DER)
	if err != nil {
		t.Fatal(err)
	}
	binario := construir(t, "dev")

	// A chave dev na pasta de configuração do Windows (`%APPDATA%`, a do `os.UserConfigDir`).
	cfg := t.TempDir()
	t.Setenv("APPDATA", cfg)
	chave := gravarChaveDev(t, filepath.Join(cfg, "confidata-assinador"), "dev-windows")

	var extensaoDev struct {
		IDChrome string `json:"idChrome"`
	}
	bruto, err := os.ReadFile(filepath.Join("..", "..", "..", "protocolo", "extensao-dev.json"))
	if err != nil || json.Unmarshal(bruto, &extensaoDev) != nil || extensaoDev.IDChrome == "" {
		t.Fatalf("protocolo/extensao-dev.json ilegível: %v", err)
	}
	janela, _, _ := win.NewLazySystemDLL("user32.dll").NewProc("GetDesktopWindow").Call()
	s := iniciarPrograma(t, binario, "chrome-extension://"+extensaoDev.IDChrome+"/", fmt.Sprintf("--parent-window=%d", janela))
	s.chave, s.kid = chave, "dev-windows"

	if r := s.pedir(t, "ola", nil); r["ok"] != true || !strings.HasPrefix(r["dados"].(map[string]any)["plataforma"].(string), "windows-") {
		t.Fatalf("ola: %v", r)
	}
	h := sha256.Sum256(c.DER)
	ref := hex.EncodeToString(h[:])
	r := s.pedir(t, "listar", nil)
	achou := false
	for _, item := range r["dados"].(map[string]any)["certificados"].([]any) {
		cm := item.(map[string]any)
		if cm["ref"] == ref {
			achou = cm["exigePin"] == false && cm["provedor"] == "windows:"+windows.CaminhoCNG && cm["rotuloDoProvedor"] == windows.RotuloInstalado
		}
	}
	if !achou {
		t.Fatalf("o certificado criado não veio como esperado: %v", r)
	}

	digest := sha256.Sum256([]byte("contrato da ponta a ponta no Windows"))
	dig := hex.EncodeToString(digest[:])
	r = s.pedir(t, "conferir", map[string]string{"ref": ref, "digest": dig, "bilhete": s.bilhete(t, dig, ref)})
	d, _ := r["dados"].(map[string]any)
	if r["ok"] != true || d["certificado"].(map[string]any)["exigePin"] != false {
		t.Fatalf("conferir: o PIN é do provedor do Windows, e a janela não mostra campo: %v", r)
	}

	// Sem PIN: quem pediria é o provedor, num diálogo do Windows (a chave de software não pede).
	r = s.pedir(t, "assinar", map[string]string{"ref": ref, "digest": dig, "bilhete": s.bilhete(t, dig, ref)})
	if r["ok"] != true {
		t.Fatalf("assinar: %v", r)
	}
	assinada, err := base64.StdEncoding.DecodeString(r["dados"].(map[string]any)["assinatura"].(string))
	if err != nil {
		t.Fatal(err)
	}
	if err := rsa.VerifyPKCS1v15(x.PublicKey.(*rsa.PublicKey), crypto.SHA256, digest[:], assinada); err != nil {
		t.Fatalf("a assinatura não confere: %v", err)
	}

	// Bilhete de outra chave: recusado antes de abrir a chave.
	outra, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	s.chave = outra
	if r = s.pedir(t, "assinar", map[string]string{"ref": ref, "digest": dig, "bilhete": s.bilhete(t, dig, ref)}); codigo(r) != "bilhete-invalido" {
		t.Fatalf("bilhete forjado: %v", r)
	}

	// O diagnóstico fala do Windows (o nome do sistema e o provedor da chave), e não do pcscd.
	r = s.pedir(t, "diagnostico", nil)
	d, _ = r["dados"].(map[string]any)
	texto, _ := d["texto"].(string)
	relatorio, _ := d["relatorio"].(map[string]any)
	if r["ok"] != true || !strings.Contains(fmt.Sprint(relatorio["sistema"]), "Windows") || !strings.Contains(texto, windowsteste.KSP) || strings.Contains(texto, "pcscd") {
		t.Fatalf("diagnostico: %v", r)
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
