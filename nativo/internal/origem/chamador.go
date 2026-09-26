package origem

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// ExtensaoFirefox é o ID fixo da extensão no Firefox (`browser_specific_settings.gecko.id`).
// Trocá-lo quebra instalações.
const ExtensaoFirefox = "assinador@confidata.com.br"

// extensoesChromePublicadas são os IDs da extensão na Chrome Web Store e no Edge Add-ons. Eles só
// existem a partir do rascunho do item na loja, e os DEFINITIVOS entram na F7a. Até lá a lista é
// vazia, e o programa de release recusa chamada do Chrome e do Edge (falha fechada).
var extensoesChromePublicadas []string

var idDoChrome = regexp.MustCompile(`^[a-p]{32}$`)

// Navegadores, pela forma como lançaram o programa.
const (
	Chromium = "chromium" // Chrome e Edge
	Firefox  = "firefox"
)

// Chamador é a extensão que o navegador diz ter lançado o programa, lida dos argumentos.
//
// O manifesto do host já restringe quem pode lançá-lo (`allowed_origins` e
// `allowed_extensions`); conferir de novo aqui protege contra manifesto trocado ou instalado por
// outro programa apontando para o nosso binário. Não protege contra um programa local que o
// execute direto com argumentos forjados: esse já alcançaria o cartão sem nós.
type Chamador struct {
	Navegador string
	Extensao  string
	// JanelaMae é o `--parent-window` que o Chrome passa no Windows, para o diálogo de PIN abrir na
	// frente (usado pelo provedor do Windows, na F6a). Zero quando não veio.
	JanelaMae uint64
}

// LerChamador interpreta os argumentos que o navegador passa ao host:
//
//   - Chrome e Edge: `chrome-extension://<id>/`, e no Windows também `--parent-window=<n>`;
//   - Firefox: o caminho do manifesto do host e o ID da extensão.
//
// Qualquer outra forma é recusada.
func LerChamador(args []string) (Chamador, bool) {
	if len(args) >= 1 && strings.HasPrefix(args[0], "chrome-extension://") {
		id, ok := strings.CutPrefix(args[0], "chrome-extension://")
		if !ok {
			return Chamador{}, false
		}
		id, ok = strings.CutSuffix(id, "/")
		if !ok || !idDoChrome.MatchString(id) {
			return Chamador{}, false
		}
		c := Chamador{Navegador: Chromium, Extensao: id}
		for _, a := range args[1:] {
			valor, ok := strings.CutPrefix(a, "--parent-window=")
			if !ok || c.JanelaMae != 0 {
				return Chamador{}, false
			}
			n, err := strconv.ParseUint(valor, 10, 64)
			if err != nil {
				return Chamador{}, false
			}
			c.JanelaMae = n
		}
		return c, true
	}
	if len(args) == 2 && args[0] != "" && args[1] != "" {
		return Chamador{Navegador: Firefox, Extensao: args[1]}, true
	}
	return Chamador{}, false
}

// Permitido diz se o chamador é uma das NOSSAS extensões.
func (c Chamador) Permitido() bool {
	switch c.Navegador {
	case Chromium:
		return slices.Contains(extensoesChromePublicadas, c.Extensao) || slices.Contains(extensoesChromeDev, c.Extensao)
	case Firefox:
		return c.Extensao == ExtensaoFirefox
	default:
		return false
	}
}
