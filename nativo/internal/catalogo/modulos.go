// Package catalogo tem os módulos PKCS#11 que alguém MEDIU (regra 4 do CLAUDE.md): cada entrada
// com a data, quem mediu e em que equipamento. Módulo novo entra quando alguém instalar o
// middleware e medir, nunca por suposição. Os ATRs de cartão entram na F2b, junto com o
// diagnóstico que os usa.
package catalogo

import "runtime"

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

var medicaoDaF0 = Medicao{
	Em:          "2026-09-25",
	Por:         "Claude (sessão da F0)",
	Equipamento: "Ubuntu 24.04 x86_64, pcscd ativo, sem cartão na leitora",
	Resultado:   "o módulo carrega e responde a C_Initialize e C_GetInfo (Cryptoki 2.20); zero slots com token",
	Registro:    "docs/medicoes/F0.md",
}

// Modulos são os medidos. O OpenSC no Fedora (`/usr/lib64/...`) e o arm64 entram quando forem
// medidos (a F2b instala o `.rpm` num Fedora em contêiner).
var Modulos = []Modulo{
	{
		Nome:   "safesign",
		Rotulo: "SafeSign",
		Caminhos: map[string][]string{
			"linux/amd64": {"/usr/lib/libaetpkss.so.3"},
		},
		Medicoes: []Medicao{medicaoDaF0},
	},
	{
		Nome:     "opensc",
		Rotulo:   "OpenSC",
		Generico: true,
		Caminhos: map[string][]string{
			"linux/amd64": {"/usr/lib/x86_64-linux-gnu/opensc-pkcs11.so"},
		},
		Medicoes: []Medicao{medicaoDaF0},
	},
}
