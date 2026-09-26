// Package assinatura tem o que vale para todo provedor de chave (PKCS#11 no Linux e no macOS, CNG e
// CSP no Windows): o DigestInfo, a interface `Provedor`, e as regras de quais certificados a lista
// mostra e quais podem assinar (§3.5 do plano).
package assinatura

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
)

// prefixoDigestInfoSHA256 é o DER de DigestInfo{ sha256, NULL } sem o OCTET STRING do resumo. Com
// `CKM_RSA_PKCS` o cartão assina exatamente o que recebe, então quem monta o DigestInfo é o
// programa.
var prefixoDigestInfoSHA256 = []byte{
	0x30, 0x31, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x65, 0x03, 0x04, 0x02, 0x01, 0x05, 0x00, 0x04, 0x20,
}

// DigestInfo monta o bloco que a chave assina com RSA PKCS#1 v1.5.
func DigestInfo(digest [32]byte) []byte {
	out := make([]byte, 0, len(prefixoDigestInfoSHA256)+len(digest))
	out = append(out, prefixoDigestInfoSHA256...)
	return append(out, digest[:]...)
}

// Ref é o identificador do certificado no protocolo: SHA-256 do DER, em hexadecimal minúsculo.
func Ref(der []byte) string {
	h := sha256.Sum256(der)
	return hex.EncodeToString(h[:])
}

// Certificado é um certificado que um provedor achou, com o que a lista precisa e o que o próprio
// provedor precisa para assinar com ele depois (`Interno`, que nunca sai do programa).
type Certificado struct {
	Ref              string
	DER              []byte
	Provedor         string
	RotuloDoProvedor string
	Leitor           string
	ExigePin         bool
	EstadoDoPin      string
	// ChaveConfirmada diz se o provedor VIU a chave privada antes do login. Há token que só mostra
	// a chave depois do PIN; ali o certificado é listado pelo uso, e `assinar` responde
	// `chave-ausente` se, depois do login, ela não existir.
	ChaveConfirmada bool
	Interno         any
}

// RelatorioDoProvedor é o que um provedor diz de si no diagnóstico. Sem CPF, sem nome de titular:
// os certificados vistos (`Vistos`) não vão ao JSON como estão; quem monta o relatório os resume
// com o nome mascarado.
type RelatorioDoProvedor struct {
	Nome         string        `json:"nome"`
	Caminho      string        `json:"caminho,omitempty"`
	Origem       string        `json:"origem,omitempty"`
	Fabricante   string        `json:"fabricante,omitempty"`
	Estado       string        `json:"estado"`
	Certificados int           `json:"certificados"`
	Detalhe      string        `json:"detalhe,omitempty"`
	Vistos       []Certificado `json:"-"`
}

// Estados de provedor no diagnóstico.
const (
	EstadoCarregado = "carregado"
	EstadoAusente   = "ausente"
	EstadoFalhou    = "falhou"
)

// RotuloChaveNoComputador é o `RotuloDoProvedor` do certificado cuja chave mora no próprio
// computador (o A1 importado no Windows, a chave no TPM), e não num cartão ou token. Hoje só o
// provedor do Windows o produz; o diagnóstico o usa para não confundir esse certificado com o do
// cartão ("o cartão está na leitora, e o certificado dele não aparece").
const RotuloChaveNoComputador = "Certificado instalado no Windows"

// Provedor é quem alcança as chaves: um por sistema (PKCS#11, Windows).
type Provedor interface {
	// Listar nunca pede PIN. Os avisos são frases para a pessoa ("o programa do cartão falhou").
	Listar(ctx context.Context) ([]Certificado, []string)
	// Assinar assina o DigestInfo SHA-256 de `digest` com a chave do certificado. O PIN (vazio no
	// caminho protegido) é zerado pelo provedor assim que ele não precisar mais dele. A recusa é
	// sempre um `*protocolo.Erro`.
	Assinar(ctx context.Context, c Certificado, digest [32]byte, pin []byte) ([]byte, error)
	Diagnosticar(ctx context.Context) []RelatorioDoProvedor
}

// Motivos pelos quais um certificado do dispositivo não entra na lista.
const (
	ForaDaListaCA        = "ca"
	ForaDaListaUso       = "uso"
	ForaDaListaAlgoritmo = "algoritmo"
)

// Listavel diz se o certificado entra na lista: não é de AC, tem `keyUsage` com
// `digitalSignature` ou `nonRepudiation`, e a chave é RSA (só RSA na v1). Vencido ENTRA: a tela diz
// "venceu em", e `Assinavel` recusa.
func Listavel(c *x509.Certificate) (bool, string) {
	if c.IsCA {
		return false, ForaDaListaCA
	}
	if c.KeyUsage&(x509.KeyUsageDigitalSignature|x509.KeyUsageContentCommitment) == 0 {
		return false, ForaDaListaUso
	}
	if _, ok := c.PublicKey.(*rsa.PublicKey); !ok {
		return false, ForaDaListaAlgoritmo
	}
	return true, ""
}

// Assinavel diz se o certificado pode assinar AGORA: listável e dentro da validade.
func Assinavel(c *x509.Certificate, agora time.Time) *protocolo.Erro {
	if ok, motivo := Listavel(c); !ok {
		if motivo == ForaDaListaAlgoritmo {
			return protocolo.Novo(protocolo.AlgoritmoNaoSuportado, "só RSA na v1")
		}
		return protocolo.Novo(protocolo.CertificadoNaoEncontrado, "certificado sem uso de assinatura")
	}
	if agora.Before(c.NotBefore) {
		return protocolo.Novo(protocolo.CertificadoNaoEncontrado, "certificado ainda não vale")
	}
	if agora.After(c.NotAfter) {
		return protocolo.Novo(protocolo.CertificadoNaoEncontrado, "certificado vencido")
	}
	return nil
}

// ConferirAssinatura confere a assinatura que o dispositivo devolveu contra a chave pública do
// certificado. Chave do token que não é a do certificado (CKA_ID trocado, cartão reescrito)
// produziria uma assinatura que ninguém confere depois; aqui ela para.
func ConferirAssinatura(c *x509.Certificate, digest [32]byte, assinatura []byte) error {
	publica, ok := c.PublicKey.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("chave do certificado não é RSA")
	}
	return rsa.VerifyPKCS1v15(publica, crypto.SHA256, digest[:], assinatura)
}

// Mascarar troca os dígitos por asterisco: o CN ICP-Brasil é `NOME:CPF` (ou `EMPRESA:CNPJ`), e o
// documento não sai do programa por texto que ele mesmo monta (regra 9 do CLAUDE.md).
func Mascarar(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteByte('*')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Zerar apaga um PIN.
func Zerar(b []byte) {
	clear(b)
}
