// Package catalogo tem os módulos PKCS#11 e os cartões (pelo ATR) que alguém MEDIU (regra 4 do
// CLAUDE.md): cada entrada com a data, quem mediu e em que equipamento. Entrada nova vem de alguém
// que instalou o middleware ou pôs o cartão na leitora e mediu, nunca de suposição nem de lista
// alheia.
package catalogo

import (
	"path/filepath"
	"runtime"
)

// Medicao é a prova de que a entrada foi vista funcionando.
type Medicao struct {
	Em          string // AAAA-MM-DD
	Por         string
	Equipamento string
	Resultado   string
	Registro    string // onde a medição está escrita
}

// Modulo é um middleware PKCS#11 conhecido.
type Modulo struct {
	// Nome é o identificador de máquina (vai em `provedor`, prefixado por `pkcs11:`).
	Nome string
	// Rotulo é o que a pessoa lê na lista e no diagnóstico.
	Rotulo string
	// Fabricante é o `manufacturerID` que o módulo declara no C_GetInfo, como foi medido. É o que
	// reconhece o módulo no diagnóstico mesmo quando ele foi achado por outro caminho (o p11-kit, a
	// configuração da pessoa) e com outro nome.
	Fabricante string
	// Generico marca o módulo que fala com muitos cartões (o OpenSC). Quando o do fabricante e um
	// genérico veem o mesmo certificado, a fusão por `ref` fica com o do fabricante.
	Generico bool
	// Caminhos por plataforma (`GOOS/GOARCH`), na ordem em que se tenta.
	Caminhos map[string][]string
	Medicoes []Medicao
}

// CaminhosAqui são os caminhos desta plataforma.
func (m Modulo) CaminhosAqui() []string {
	return m.Caminhos[runtime.GOOS+"/"+runtime.GOARCH]
}

// PeloArquivo acha no catálogo o módulo cuja biblioteca tem este NOME de arquivo, em qualquer
// plataforma: o SafeSign instalado fora do caminho medido, ou o OpenSC que o p11-kit registra,
// continuam sendo o SafeSign e o OpenSC (com o rótulo e o `Generico` do catálogo).
func PeloArquivo(caminho string, modulos []Modulo) (Modulo, bool) {
	nome := filepath.Base(caminho)
	for _, m := range modulos {
		for _, caminhos := range m.Caminhos {
			for _, c := range caminhos {
				if filepath.Base(c) == nome {
					return m, true
				}
			}
		}
	}
	return Modulo{}, false
}

var medicaoDaF0 = Medicao{
	Em:          "2026-09-25",
	Por:         "Claude (sessão da F0)",
	Equipamento: "Ubuntu 24.04 x86_64, pcscd ativo, sem cartão na leitora",
	Resultado:   "o módulo carrega e responde a C_Initialize e C_GetInfo (Cryptoki 2.20); zero slots com token",
	Registro:    "docs/medicoes/F0.md",
}

var medicaoDoOpenscNoFedora = Medicao{
	Em:          "2026-09-26",
	Por:         "Claude (sessão da F2b)",
	Equipamento: "Fedora 43 x86_64 em contêiner, OpenSC 0.27.1 e p11-kit 0.26.5 do dnf, sem leitora",
	Resultado:   "o módulo fica em /usr/lib64/opensc-pkcs11.so (e o p11-kit o registra por nome, em /usr/lib64/pkcs11) e responde a C_GetInfo (Cryptoki 3.0); zero slots",
	Registro:    "docs/medicoes/F2b.md",
}

// Modulos são os medidos. O arm64 entra quando for medido.
var Modulos = []Modulo{
	{
		Nome:       "safesign",
		Rotulo:     "SafeSign",
		Fabricante: "A.E.T. Europe B.V.",
		Caminhos: map[string][]string{
			"linux/amd64": {"/usr/lib/libaetpkss.so.3"},
		},
		Medicoes: []Medicao{medicaoDaF0},
	},
	{
		Nome:       "opensc",
		Rotulo:     "OpenSC",
		Fabricante: "OpenSC Project",
		Generico:   true,
		Caminhos: map[string][]string{
			"linux/amd64": {"/usr/lib/x86_64-linux-gnu/opensc-pkcs11.so", "/usr/lib64/opensc-pkcs11.so"},
		},
		Medicoes: []Medicao{medicaoDaF0, medicaoDoOpenscNoFedora},
	},
}

// ATR é um cartão cujo ATR alguém MEDIU, com o módulo do catálogo que o lê. É o que deixa o
// diagnóstico dizer "este cartão usa o SafeSign" quando o SafeSign não está instalado.
type ATR struct {
	// Valor é o ATR como a leitora o devolve: hexadecimal maiúsculo, sem espaço.
	Valor string
	// Cartao é o que a pessoa e o suporte leem ("cartão Certisign").
	Cartao string
	// Modulo é o `Nome` do módulo do catálogo que lê este cartão.
	Modulo   string
	Medicoes []Medicao
}

// ATRs são os cartões medidos. O do cartão Certisign do Cairo entra quando ele for lido com o
// `assinador diagnostico` (o gate da F2b, em `docs/medicoes/F2b.md`).
var ATRs = []ATR{}

// ModuloDoATR acha, pelo ATR medido, o módulo que lê o cartão.
func ModuloDoATR(atr string, atrs []ATR, modulos []Modulo) (Modulo, ATR, bool) {
	for _, a := range atrs {
		if a.Valor != atr {
			continue
		}
		for _, m := range modulos {
			if m.Nome == a.Modulo {
				return m, a, true
			}
		}
	}
	return Modulo{}, ATR{}, false
}
