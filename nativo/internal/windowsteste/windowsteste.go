//go:build windows

// Package windowsteste cria, para os TESTES, certificados autoassinados com chave de SOFTWARE no
// repositório pessoal do usuário do Windows (`CurrentUser\My`), pelo `New-SelfSignedCertificate`:
// é o papel que o SoftHSM2 faz no Linux. Não entra no programa: só arquivos `_test.go` o importam
// (a catraca de `cmd/assinador/release_test.go` confere o binário). Cada certificado sai do
// repositório, com a chave, no fim do teste.
//
// Sem o PowerShell ou sem o módulo PKI, o teste é PULADO, a não ser que
// `ASSINADOR_EXIGE_WINDOWS=1` (o CI liga), e aí ele reprova.
package windowsteste

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

// Provedores de chave de software do Windows, um por caminho do programa.
const (
	// KSP é o caminho CNG.
	KSP = "Microsoft Software Key Storage Provider"
	// CSP é o caminho CryptoAPI legado, com SHA-256.
	CSP = "Microsoft Enhanced RSA and AES Cryptographic Provider"
	// CSPSemSHA256 é o CSP legado que não tem SHA-256: o programa responde algoritmo-nao-suportado.
	CSPSemSHA256 = "Microsoft Base Cryptographic Provider v1.0"
)

// Certificado é o certificado criado: o DER e a impressão (SHA-1, como o Windows a mostra).
type Certificado struct {
	DER       []byte
	Impressao string
}

var impressao = regexp.MustCompile(`^[0-9A-F]{40}$`)

// Novo cria um certificado com chave no provedor pedido e o apaga (com a chave) no fim do teste.
func Novo(t *testing.T, provedor string) Certificado {
	t.Helper()
	c, err := Tentar(t, provedor)
	if err != nil {
		if os.Getenv("ASSINADOR_EXIGE_WINDOWS") == "1" {
			t.Fatal(err)
		}
		t.Skip(err)
	}
	return c
}

// Tentar é o `Novo` que devolve o erro, para o provedor que pode não existir na máquina.
func Tentar(t *testing.T, provedor string) (Certificado, error) {
	t.Helper()
	sufixo := make([]byte, 4)
	_, _ = rand.Read(sufixo)
	var roteiro strings.Builder
	roteiro.WriteString("$ErrorActionPreference = 'Stop'\n")
	fmt.Fprintf(&roteiro, "$p = @{ Subject = 'CN=ASSINADOR TESTE %s'; CertStoreLocation = 'Cert:\\CurrentUser\\My'; KeyAlgorithm = 'RSA'; KeyLength = 2048; KeyUsage = @('DigitalSignature','NonRepudiation'); Provider = '%s'; KeyExportPolicy = 'NonExportable'; NotAfter = (Get-Date).AddDays(2) }\n", hex.EncodeToString(sufixo), provedor)
	if provedor != KSP {
		// Chave de CSP legado assina pela chave de ASSINATURA (AT_SIGNATURE).
		roteiro.WriteString("$p.KeySpec = 'Signature'\n")
	}
	if provedor == CSPSemSHA256 {
		// O próprio certificado é assinado pelo provedor, que não tem SHA-256.
		roteiro.WriteString("$p.HashAlgorithm = 'SHA1'\n")
	}
	roteiro.WriteString("$c = New-SelfSignedCertificate @p\n")
	roteiro.WriteString("[Convert]::ToBase64String($c.RawData) + ' ' + $c.Thumbprint\n")
	saida, err := powershell(roteiro.String())
	campos := strings.Fields(saida)
	if err != nil || len(campos) != 2 || !impressao.MatchString(campos[1]) {
		return Certificado{}, fmt.Errorf("o New-SelfSignedCertificate não criou o certificado (%s): %v %q", provedor, err, saida)
	}
	c := Certificado{Impressao: campos[1]}
	t.Cleanup(func() {
		if _, err := powershell("Remove-Item -Path 'Cert:\\CurrentUser\\My\\" + c.Impressao + "' -DeleteKey"); err != nil {
			t.Logf("o certificado de teste %s ficou no repositório: %v", c.Impressao, err)
		}
	})
	if c.DER, err = base64.StdEncoding.DecodeString(campos[0]); err != nil {
		return Certificado{}, fmt.Errorf("DER ilegível: %w", err)
	}
	return c, nil
}

func powershell(roteiro string) (string, error) {
	out, err := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", roteiro).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
