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
	c, err := criar(t, provedor)
	if err != nil {
		if os.Getenv("ASSINADOR_EXIGE_WINDOWS") == "1" {
			t.Fatal(err)
		}
		t.Skip(err)
	}
	return c
}

// criar monta o certificado. O assunto leva um sufixo aleatório, e a limpeza é registrada por ELE,
// antes de a saída ser lida: o certificado sai do repositório mesmo quando a saída não se lê.
func criar(t *testing.T, provedor string) (Certificado, error) {
	t.Helper()
	sufixo := make([]byte, 4)
	_, _ = rand.Read(sufixo)
	assunto := "CN=ASSINADOR TESTE " + hex.EncodeToString(sufixo)
	t.Cleanup(func() {
		roteiro := fmt.Sprintf("Get-ChildItem -Path 'Cert:\\CurrentUser\\My' | Where-Object { $_.Subject -eq '%s' } | Remove-Item -DeleteKey", assunto)
		if _, err := powershell(roteiro); err != nil {
			t.Logf("o certificado de teste %q ficou no repositório: %v", assunto, err)
		}
	})
	var roteiro strings.Builder
	fmt.Fprintf(&roteiro, "$p = @{ Subject = '%s'; CertStoreLocation = 'Cert:\\CurrentUser\\My'; KeyAlgorithm = 'RSA'; KeyLength = 2048; KeyUsage = @('DigitalSignature','NonRepudiation'); Provider = '%s'; KeyExportPolicy = 'NonExportable'; NotAfter = (Get-Date).AddDays(2) }\n", assunto, provedor)
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
	// A última linha é a do certificado: o que o PowerShell escreveu antes (aviso, progresso) não conta.
	linhas := strings.Split(saida, "\n")
	campos := strings.Fields(linhas[len(linhas)-1])
	if err != nil || len(campos) != 2 || !impressao.MatchString(campos[1]) {
		return Certificado{}, fmt.Errorf("o New-SelfSignedCertificate não criou o certificado (%s): %v %q", provedor, err, saida)
	}
	c := Certificado{Impressao: campos[1]}
	if c.DER, err = base64.StdEncoding.DecodeString(campos[0]); err != nil {
		return Certificado{}, fmt.Errorf("DER ilegível: %w", err)
	}
	return c, nil
}

// powershell roda o roteiro e devolve a SAÍDA PADRÃO, sem as linhas em branco nas pontas. O erro e o
// progresso não entram nela: o progresso do PowerShell 5.1 (a preparação dos módulos na primeira
// vez) sai em CLIXML no erro padrão, e aqui ele nem é gerado.
func powershell(roteiro string) (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", "$ErrorActionPreference = 'Stop'\n$ProgressPreference = 'SilentlyContinue'\n"+roteiro)
	var erro strings.Builder
	cmd.Stderr = &erro
	out, err := cmd.Output()
	if err != nil {
		err = fmt.Errorf("%w: %s", err, strings.TrimSpace(erro.String()))
	}
	return strings.TrimSpace(strings.ReplaceAll(string(out), "\r\n", "\n")), err
}
