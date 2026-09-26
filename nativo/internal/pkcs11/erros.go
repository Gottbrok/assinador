//go:build !windows

package pkcs11

import (
	"errors"
	"strings"

	p11 "github.com/miekg/pkcs11"

	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
)

// nomeDoErro é o nome `CKR_*` de um erro do módulo, para o detalhe (que vai ao suporte).
func nomeDoErro(err error) string {
	var e p11.Error
	if errors.As(err, &e) {
		// "pkcs11: 0x000000A0: CKR_PIN_INCORRECT"
		if i := strings.LastIndex(e.Error(), "CKR_"); i >= 0 {
			return e.Error()[i:]
		}
		return e.Error()
	}
	return err.Error()
}

func codigoCkr(err error) (p11.Error, bool) {
	var e p11.Error
	ok := errors.As(err, &e)
	return e, ok
}

// removido: o cartão saiu da leitora ou a sessão caiu no meio.
func removido(e p11.Error) bool {
	switch e {
	case p11.CKR_DEVICE_REMOVED, p11.CKR_TOKEN_NOT_PRESENT, p11.CKR_SESSION_HANDLE_INVALID, p11.CKR_SESSION_CLOSED, p11.CKR_TOKEN_NOT_RECOGNIZED:
		return true
	}
	return false
}

// tentativasDasFlags lê, das flags do token, quantas tentativas de PIN restam.
func tentativasDasFlags(flags uint) string {
	switch {
	case flags&p11.CKF_USER_PIN_FINAL_TRY != 0:
		return protocolo.TentativasUltima
	case flags&p11.CKF_USER_PIN_COUNT_LOW != 0:
		return protocolo.TentativasPoucas
	}
	return ""
}

// estadoDoPin é o `estadoDoPin` da lista.
func estadoDoPin(flags uint) string {
	switch {
	case flags&p11.CKF_USER_PIN_LOCKED != 0:
		return protocolo.PinBloqueado
	case flags&p11.CKF_USER_PIN_FINAL_TRY != 0:
		return protocolo.PinUltimaTentativa
	case flags&p11.CKF_USER_PIN_COUNT_LOW != 0:
		return protocolo.PinPoucasTentativas
	}
	return protocolo.PinOk
}

// erroDoLogin traduz a falha de um C_Login. `flagsDepois` são as flags do token RELIDAS depois da
// falha (o token só atualiza a contagem depois da tentativa): o PIN errado que bloqueou o cartão é
// `token-bloqueado`, não `pin-incorreto`.
func erroDoLogin(err error, flagsDepois uint) *protocolo.Erro {
	e, ok := codigoCkr(err)
	if !ok {
		return protocolo.Novo(protocolo.ModuloFalhou, "C_Login: "+err.Error())
	}
	switch {
	case e == p11.CKR_PIN_INCORRECT || e == p11.CKR_PIN_INVALID || e == p11.CKR_PIN_LEN_RANGE:
		if flagsDepois&p11.CKF_USER_PIN_LOCKED != 0 {
			return protocolo.Novo(protocolo.TokenBloqueado, "C_Login: "+nomeDoErro(err))
		}
		return protocolo.PinErrado(tentativasDasFlags(flagsDepois))
	case e == p11.CKR_PIN_LOCKED || e == p11.CKR_PIN_EXPIRED:
		return protocolo.Novo(protocolo.TokenBloqueado, "C_Login: "+nomeDoErro(err))
	case e == p11.CKR_FUNCTION_CANCELED:
		return protocolo.Novo(protocolo.Cancelado, "C_Login: "+nomeDoErro(err))
	case removido(e):
		return protocolo.Novo(protocolo.CertificadoNaoEncontrado, "o dispositivo saiu no meio: "+nomeDoErro(err))
	}
	return protocolo.Novo(protocolo.ModuloFalhou, "C_Login: "+nomeDoErro(err))
}

// erroDaAssinatura traduz a falha de C_SignInit ou C_Sign.
func erroDaAssinatura(etapa string, err error) *protocolo.Erro {
	e, ok := codigoCkr(err)
	if !ok {
		return protocolo.Novo(protocolo.ModuloFalhou, etapa+": "+err.Error())
	}
	switch {
	case e == p11.CKR_MECHANISM_INVALID || e == p11.CKR_KEY_TYPE_INCONSISTENT || e == p11.CKR_KEY_FUNCTION_NOT_PERMITTED || e == p11.CKR_MECHANISM_PARAM_INVALID:
		return protocolo.Novo(protocolo.AlgoritmoNaoSuportado, etapa+": "+nomeDoErro(err))
	case e == p11.CKR_FUNCTION_CANCELED:
		return protocolo.Novo(protocolo.Cancelado, etapa+": "+nomeDoErro(err))
	case e == p11.CKR_PIN_LOCKED:
		return protocolo.Novo(protocolo.TokenBloqueado, etapa+": "+nomeDoErro(err))
	case removido(e):
		return protocolo.Novo(protocolo.CertificadoNaoEncontrado, "o dispositivo saiu no meio: "+nomeDoErro(err))
	}
	return protocolo.Novo(protocolo.ModuloFalhou, etapa+": "+nomeDoErro(err))
}
