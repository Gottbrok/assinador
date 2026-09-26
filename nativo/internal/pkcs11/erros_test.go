//go:build !windows

package pkcs11

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"testing"

	p11 "github.com/miekg/pkcs11"

	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
)

// O mapa das flags e dos CKR_* para o protocolo. É por aqui que o bloqueio e a última tentativa se
// provam: o SoftHSM marca as poucas tentativas (ver `TestAssinarComPinErradoNaoRepete`), mas nunca
// bloqueia.
func TestErroDoLogin(t *testing.T) {
	casos := []struct {
		nome       string
		err        error
		flags      uint
		codigo     protocolo.Codigo
		tentativas string
	}{
		{"pin errado", p11.Error(p11.CKR_PIN_INCORRECT), 0, protocolo.PinIncorreto, ""},
		{"pin errado, poucas", p11.Error(p11.CKR_PIN_INCORRECT), p11.CKF_USER_PIN_COUNT_LOW, protocolo.PinIncorreto, protocolo.TentativasPoucas},
		{"pin errado, última", p11.Error(p11.CKR_PIN_INCORRECT), p11.CKF_USER_PIN_COUNT_LOW | p11.CKF_USER_PIN_FINAL_TRY, protocolo.PinIncorreto, protocolo.TentativasUltima},
		{"pin errado que bloqueou", p11.Error(p11.CKR_PIN_INCORRECT), p11.CKF_USER_PIN_LOCKED, protocolo.TokenBloqueado, ""},
		{"pin fora do tamanho", p11.Error(p11.CKR_PIN_LEN_RANGE), 0, protocolo.PinIncorreto, ""},
		{"bloqueado", p11.Error(p11.CKR_PIN_LOCKED), 0, protocolo.TokenBloqueado, ""},
		{"cancelado no leitor", p11.Error(p11.CKR_FUNCTION_CANCELED), 0, protocolo.Cancelado, ""},
		{"cartão removido", p11.Error(p11.CKR_DEVICE_REMOVED), 0, protocolo.CertificadoNaoEncontrado, ""},
		{"outro", p11.Error(p11.CKR_GENERAL_ERROR), 0, protocolo.ModuloFalhou, ""},
		{"fora do pkcs11", errors.New("x"), 0, protocolo.ModuloFalhou, ""},
	}
	for _, c := range casos {
		e := erroDoLogin(c.err, c.flags)
		if e.Codigo != c.codigo || e.Tentativas != c.tentativas {
			t.Errorf("%s: %+v", c.nome, e)
		}
	}
}

func TestErroDaAssinatura(t *testing.T) {
	casos := map[uint]protocolo.Codigo{
		p11.CKR_MECHANISM_INVALID:          protocolo.AlgoritmoNaoSuportado,
		p11.CKR_KEY_TYPE_INCONSISTENT:      protocolo.AlgoritmoNaoSuportado,
		p11.CKR_KEY_FUNCTION_NOT_PERMITTED: protocolo.AlgoritmoNaoSuportado,
		p11.CKR_FUNCTION_CANCELED:          protocolo.Cancelado,
		p11.CKR_TOKEN_NOT_PRESENT:          protocolo.CertificadoNaoEncontrado,
		p11.CKR_DEVICE_ERROR:               protocolo.ModuloFalhou,
	}
	for ckr, codigo := range casos {
		if e := erroDaAssinatura("C_Sign", p11.Error(ckr)); e.Codigo != codigo {
			t.Errorf("%x: %v", ckr, e)
		}
	}
}

// O código que o filho manda só passa se for do vocabulário, e as tentativas só com os dois valores.
func TestErroDaRespostaConfereOVocabulario(t *testing.T) {
	e := erroDaResposta(respostaDoModulo{Erro: &erroDoModulo{Codigo: "inventado", Detalhe: "x"}})
	if e.Codigo != protocolo.Interno {
		t.Fatalf("código fora do vocabulário passou: %+v", e)
	}
	e = erroDaResposta(respostaDoModulo{Erro: &erroDoModulo{Codigo: protocolo.PinIncorreto, Tentativas: "muitas"}})
	if e.Codigo != protocolo.PinIncorreto || e.Tentativas != "" {
		t.Fatalf("tentativas fora do vocabulário passaram: %+v", e)
	}
	e = erroDaResposta(respostaDoModulo{Erro: &erroDoModulo{Codigo: protocolo.PinIncorreto, Tentativas: protocolo.TentativasUltima}})
	if e.Tentativas != protocolo.TentativasUltima {
		t.Fatalf("%+v", e)
	}
	if e := erroDaResposta(respostaDoModulo{}); e.Codigo != protocolo.Interno {
		t.Fatalf("resposta vazia: %+v", e)
	}
}

// O CKA_ID só vale se a chave não desmentir o certificado: no cartão renovado que manteve o ID da
// chave antiga, a chave nova se acha pelo módulo.
func TestCasarNaoAceitaIdDesmentidoPeloModulo(t *testing.T) {
	certKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	velha, _ := rsa.GenerateKey(rand.Reader, 2048)
	c := certificadoNoSlot{id: []byte{1}, cert: &x509.Certificate{PublicKey: &certKey.PublicKey}}
	chaves := []chavePrivada{
		{handle: 10, id: []byte{1}, modulo: velha.N.Bytes()},
		{handle: 20, id: []byte{2}, modulo: append([]byte{0}, certKey.N.Bytes()...)},
	}
	if k, ok := casar(c, chaves); !ok || k.handle != 20 {
		t.Fatalf("casou com %d (%v)", k.handle, ok)
	}
	// Sem módulo legível, o CKA_ID segue valendo (o token que não deixa ler o módulo da privada).
	chaves[0].modulo = nil
	if k, ok := casar(c, chaves[:1]); !ok || k.handle != 10 {
		t.Fatalf("sem módulo: %d (%v)", k.handle, ok)
	}
	// Ninguém com o ID nem com o módulo: sem chave.
	if _, ok := casar(c, []chavePrivada{{handle: 30, id: []byte{9}, modulo: velha.N.Bytes()}}); ok {
		t.Fatal("casou com chave alheia")
	}
}

func TestEstadoDoPin(t *testing.T) {
	casos := map[uint]string{
		0:                          protocolo.PinOk,
		p11.CKF_USER_PIN_COUNT_LOW: protocolo.PinPoucasTentativas,
		p11.CKF_USER_PIN_COUNT_LOW | p11.CKF_USER_PIN_FINAL_TRY: protocolo.PinUltimaTentativa,
		p11.CKF_USER_PIN_LOCKED | p11.CKF_USER_PIN_FINAL_TRY:    protocolo.PinBloqueado,
	}
	for flags, quer := range casos {
		if got := estadoDoPin(flags); got != quer {
			t.Errorf("%x: %s", flags, got)
		}
	}
}
