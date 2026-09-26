//go:build !windows

package pkcs11

import (
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
