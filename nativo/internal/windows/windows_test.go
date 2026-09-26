package windows

import (
	"bytes"
	"testing"

	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
)

// Cada recusa do Windows vira o código do protocolo que a página entende (§3.5 do plano).
func TestErroDoWindows(t *testing.T) {
	casos := []struct {
		codigo   uint32
		esperado protocolo.Codigo
	}{
		{scardCanceladoPeloUsuario, protocolo.Cancelado},
		{scardCancelado, protocolo.Cancelado},
		{nteCanceladoPeloUsuario, protocolo.Cancelado},
		{1223, protocolo.Cancelado}, // ERROR_CANCELLED cru, do GetLastError
		{erroCancelado, protocolo.Cancelado},
		{scardPinErrado, protocolo.PinIncorreto},
		{scardPinBloqueado, protocolo.TokenBloqueado},
		{nteAlgoritmo, protocolo.AlgoritmoNaoSuportado},
		{nteNaoSuportado, protocolo.AlgoritmoNaoSuportado},
		{scardSemCartao, protocolo.ChaveAusente},
		{scardCartaoRemovido, protocolo.ChaveAusente},
		{nteSemChave, protocolo.ChaveAusente},
		{nteConjuntoRuim, protocolo.ChaveAusente},
		{nteConjuntoNaoDefinido, protocolo.ChaveAusente},
		{cryptSemChave, protocolo.ChaveAusente},
		{ntePermissao, protocolo.PermissaoNegada},
		{5, protocolo.PermissaoNegada},       // ERROR_ACCESS_DENIED cru
		{0x80090020, protocolo.ModuloFalhou}, // NTE_FAIL: o provedor do fabricante falhou
		{0x12345678, protocolo.ModuloFalhou},
	}
	for _, c := range casos {
		e := erroDoWindows("assinar", c.codigo)
		if e.Codigo != c.esperado {
			t.Errorf("0x%08X: %s, e não %s", c.codigo, e.Codigo, c.esperado)
		}
	}
}

// O PIN errado vai sem tentativas (o Windows não as informa), e o detalhe das demais recusas leva a
// etapa e o código, em hexadecimal, para o suporte.
func TestDetalheDoErroDoWindows(t *testing.T) {
	if e := erroDoWindows("assinar", scardPinErrado); e.Tentativas != "" || e.DetalheParaResposta() != nil {
		t.Fatalf("pin: %+v", e)
	}
	if e := erroDoWindows("abrir a chave", 1223); e.Detalhe != "abrir a chave: 0x800704C7" {
		t.Fatalf("detalhe: %q", e.Detalhe)
	}
}

func TestInverter(t *testing.T) {
	for _, c := range []struct{ entrada, esperado []byte }{
		{[]byte{}, []byte{}},
		{[]byte{1}, []byte{1}},
		{[]byte{1, 2}, []byte{2, 1}},
		{[]byte{1, 2, 3, 4, 5}, []byte{5, 4, 3, 2, 1}},
	} {
		b := bytes.Clone(c.entrada)
		inverter(b)
		if !bytes.Equal(b, c.esperado) {
			t.Errorf("%v: %v", c.entrada, b)
		}
	}
}

// A chave que mora no computador aparece como "instalado no Windows"; a do cartão, pelo nome do
// provedor.
func TestRotuloDoProvedor(t *testing.T) {
	casos := map[string]string{
		"Microsoft Software Key Storage Provider":               RotuloInstalado,
		"Microsoft Enhanced RSA and AES Cryptographic Provider": RotuloInstalado,
		"Microsoft Platform Crypto Provider":                    RotuloInstalado,
		"Microsoft Smart Card Key Storage Provider":             "Microsoft Smart Card Key Storage Provider",
		"":                                    RotuloSemNome,
		"Provedor de um fabricante de cartão": "Provedor de um fabricante de cartão",
	}
	for nome, esperado := range casos {
		if got := rotuloDoProvedor(nome); got != esperado {
			t.Errorf("%q: %q, e não %q", nome, got, esperado)
		}
	}
	if caminhoDoTipo(0) != CaminhoCNG || caminhoDoTipo(24) != CaminhoCSP || caminhoDoTipo(1) != CaminhoCSP {
		t.Fatal("caminho pelo tipo do provedor")
	}
}
