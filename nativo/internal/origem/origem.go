// Package origem decide QUEM pode falar com o programa: a origem da página (padrões por emissor
// de bilhete, §3.4 do plano) e o chamador (a extensão que o navegador lançou).
//
// Os padrões são os de `PADROES_DE_ORIGEM` e `PADROES_DE_ORIGEM_DEV` da biblioteca
// `@confidata/icp-brasil`, e `origem_test.go` os compara com `protocolo.json` das fixtures.
package origem

import (
	"regexp"
	"slices"
)

// Emissores de bilhete. A chave pinada diz de qual emissor ela é.
const (
	Confidata = "confidata"
	Ushield   = "ushield"
)

// Emissores é a lista, na ordem das fixtures.
var Emissores = []string{Confidata, Ushield}

// Ambientes de chave de bilhete. Só `dev` acrescenta `localhost` aos padrões.
const (
	AmbienteProducao = "producao"
	AmbienteDev      = "dev"
	AmbienteTeste    = "teste"
)

// Padroes são as origens que cada emissor pode pôr em `aud`, ancoradas. Os ccTLDs do Confidata
// redirecionam 301 para o `.app`, e a Academy no `.com.br` não assina.
var Padroes = map[string][]string{
	Confidata: {`^https://[a-z0-9-]{1,63}\.confidata\.app$`},
	Ushield:   {`^https://ushield\.app$`},
}

// PadroesDev são acrescentados aos do emissor SÓ para chave de ambiente `dev`.
var PadroesDev = []string{`^http://([a-z0-9-]+\.)?localhost(:[0-9]+)?$`}

// DaExtensao é a origem que a extensão informa quando quem pergunta é uma página DELA (a de
// opções pede as versões e o diagnóstico). `.invalid` é reservado: nunca é página de verdade, e
// nenhum padrão de emissor a aceita (bilhete para ela é recusado). A extensão a informa só depois
// de conferir, pelo remetente, que o pedido veio de página dela (`ORIGEM_DA_EXTENSAO` na extensão).
const DaExtensao = "https://extensao.invalid"

var (
	regexDoEmissor = map[string][]*regexp.Regexp{}
	regexDev       []*regexp.Regexp
)

func init() {
	for emissor, padroes := range Padroes {
		for _, p := range padroes {
			regexDoEmissor[emissor] = append(regexDoEmissor[emissor], regexp.MustCompile(p))
		}
	}
	for _, p := range PadroesDev {
		regexDev = append(regexDev, regexp.MustCompile(p))
	}
}

// Aceita diz se `origem` está entre as que o emissor pode pôr em `aud`. Chave de ambiente `dev`
// acrescenta `localhost`. É a mesma regra de `origemAceita` da biblioteca.
func Aceita(origem, emissor, ambiente string) bool {
	padroes, ok := regexDoEmissor[emissor]
	if !ok {
		return false
	}
	if casa(padroes, origem) {
		return true
	}
	return ambiente == AmbienteDev && casa(regexDev, origem)
}

// PermitidaSemBilhete diz se uma página nesta origem pode pedir `listar` e `diagnostico`, que não
// têm bilhete: a de algum emissor, e `localhost` só no build de desenvolvimento. É defesa em
// profundidade: a extensão já só atua nesses endereços, e só para host que a pessoa autorizou.
func PermitidaSemBilhete(origem string) bool {
	for _, emissor := range Emissores {
		if casa(regexDoEmissor[emissor], origem) {
			return true
		}
	}
	return localhostSemBilhete && casa(regexDev, origem)
}

func casa(padroes []*regexp.Regexp, origem string) bool {
	return slices.ContainsFunc(padroes, func(r *regexp.Regexp) bool { return r.MatchString(origem) })
}
