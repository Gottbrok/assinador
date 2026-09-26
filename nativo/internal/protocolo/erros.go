// Package protocolo tem o vocabulário que a extensão e o programa compartilham: a versão, as
// operações, os limites e os códigos de erro.
//
// Os códigos são os de `@confidata/icp-brasil` (`src/browser/assinadorProtocolo.ts`,
// `CODIGOS_DE_ERRO`), e `erros_test.go` os compara com a lista das fixtures
// (`protocolo/fixtures/bilhete/protocolo.json`): mudar um lado sem o outro quebra o teste, de
// propósito. Código novo nasce na biblioteca, com fixture, e só então entra aqui.
package protocolo

import "fmt"

// Codigo é um código de erro do protocolo.
type Codigo string

const (
	OrigemRecusada           Codigo = "origem-recusada"
	BilheteInvalido          Codigo = "bilhete-invalido"
	BilheteExpirado          Codigo = "bilhete-expirado"
	Relogio                  Codigo = "relogio"
	DigestDivergente         Codigo = "digest-divergente"
	CertificadoDivergente    Codigo = "certificado-divergente"
	CertificadoNaoEncontrado Codigo = "certificado-nao-encontrado"
	ChaveAusente             Codigo = "chave-ausente"
	AlgoritmoNaoSuportado    Codigo = "algoritmo-nao-suportado"
	PermissaoNegada          Codigo = "permissao-negada"
	PinIncorreto             Codigo = "pin-incorreto"
	TokenBloqueado           Codigo = "token-bloqueado"
	Cancelado                Codigo = "cancelado"
	TempoEsgotado            Codigo = "tempo-esgotado"
	Ocupado                  Codigo = "ocupado"
	NativoAusente            Codigo = "nativo-ausente"
	NativoDesatualizado      Codigo = "nativo-desatualizado"
	ModuloFalhou             Codigo = "modulo-falhou"
	Protocolo                Codigo = "protocolo"
	Interno                  Codigo = "interno"
)

// Codigos é o conjunto do protocolo. A ordem não significa nada; o conjunto é o contrato.
var Codigos = []Codigo{
	OrigemRecusada,
	BilheteInvalido,
	BilheteExpirado,
	Relogio,
	DigestDivergente,
	CertificadoDivergente,
	CertificadoNaoEncontrado,
	ChaveAusente,
	AlgoritmoNaoSuportado,
	PermissaoNegada,
	PinIncorreto,
	TokenBloqueado,
	Cancelado,
	TempoEsgotado,
	Ocupado,
	NativoAusente,
	NativoDesatualizado,
	ModuloFalhou,
	Protocolo,
	Interno,
}

// Tentativas de PIN que restam, quando o token diz (flags do PKCS#11). Vazio quando ele não diz.
const (
	TentativasPoucas = "poucas"
	TentativasUltima = "ultima"
)

// Erro é a recusa de uma operação: o código do protocolo e, para o suporte, um detalhe em texto.
// Em `pin-incorreto` o detalhe é `{ tentativas }` (o que a página lê), e não texto.
//
// 🚫 Detalhe nunca leva PIN, CPF, nome do titular nem o conteúdo de uma mensagem: ele chega à
// página e ao diagnóstico que a pessoa copia para o suporte.
type Erro struct {
	Codigo     Codigo
	Detalhe    string
	Tentativas string
}

func (e *Erro) Error() string {
	if e.Detalhe == "" {
		return string(e.Codigo)
	}
	return fmt.Sprintf("%s: %s", e.Codigo, e.Detalhe)
}

// Novo monta uma recusa com detalhe para o suporte.
func Novo(c Codigo, detalhe string) *Erro {
	return &Erro{Codigo: c, Detalhe: detalhe}
}

// PinErrado monta o `pin-incorreto`, com as tentativas quando o token as informa.
func PinErrado(tentativas string) *Erro {
	return &Erro{Codigo: PinIncorreto, Tentativas: tentativas}
}

// DetalheParaResposta é o `detalhe` que vai no JSON: `{ tentativas }` no PIN errado (ou nada, se
// o token não disse), texto nos demais (ou nada).
func (e *Erro) DetalheParaResposta() any {
	if e.Codigo == PinIncorreto {
		if e.Tentativas == "" {
			return nil
		}
		return map[string]string{"tentativas": e.Tentativas}
	}
	if e.Detalhe == "" {
		return nil
	}
	return e.Detalhe
}
