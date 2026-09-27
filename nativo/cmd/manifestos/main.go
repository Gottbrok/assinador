// Comando manifestos: escreve os manifestos de native messaging dos navegadores, que o pacote
// instala.
//
//	go run -tags dev ./cmd/manifestos -saida <pasta> -programa /usr/lib/confidata-assinador/assinador
//	go run -tags dev ./cmd/manifestos -saida <pasta> -programa assinador.exe -relativo
//
// O caminho do programa é ABSOLUTO, ou, com `-relativo`, só o NOME do `.exe`, na mesma pasta do
// manifesto: é o que o MSI do Windows usa (F6b), porque a pasta por usuário só existe na hora da
// instalação, e o Chrome, o Edge e o Firefox resolvem caminho relativo à pasta do manifesto no Windows
// (no Linux e no macOS eles exigem absoluto).
//
// Escreve `chromium.json` (Chrome, Chromium e Edge) e `firefox.json`. O `allowed_origins` sai de
// `origem.ExtensoesChrome()` e o `allowed_extensions` de `origem.ExtensaoFirefox`: as MESMAS listas
// com que o programa confere quem o chama, então o manifesto e o programa não divergem. Com a tag
// `dev`, os IDs são os de desenvolvimento; sem ela, os das lojas (que só existem a partir da F7a:
// antes disso, o comando recusa, em vez de gerar manifesto que não deixa extensão nenhuma entrar).
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Gottbrok/assinador/nativo/internal/origem"
)

// descricao leva o nome visível (Assinador uShield, decisão D2); o `name` do host é nome interno fixo.
const descricao = "Assinador uShield: assina com o certificado digital do cartão ou do token"

type manifestoChromium struct {
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Path           string   `json:"path"`
	Type           string   `json:"type"`
	AllowedOrigins []string `json:"allowed_origins"`
}

type manifestoFirefox struct {
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	Path              string   `json:"path"`
	Type              string   `json:"type"`
	AllowedExtensions []string `json:"allowed_extensions"`
}

func main() {
	saida := flag.String("saida", "", "pasta onde escrever chromium.json e firefox.json")
	programa := flag.String("programa", "", "caminho ABSOLUTO do programa instalado (ou só o nome do .exe, com -relativo)")
	relativo := flag.Bool("relativo", false, "o -programa fica na MESMA pasta do manifesto e vai só pelo nome do .exe (Windows, MSI)")
	flag.Parse()
	if err := gerar(*saida, *programa, *relativo); err != nil {
		fmt.Fprintln(os.Stderr, "manifestos:", err)
		os.Exit(1)
	}
}

// nomeDePrograma é o que `-relativo` aceita: o NOME de um `.exe`, em lista branca. Fica de fora pasta,
// `..`, separador de qualquer sistema, letra de unidade (`C:assinador.exe` é relativo à pasta corrente
// da unidade C, e não à do manifesto), espaço e ponto no fim (que o Windows apaga do nome), caractere
// de controle e os proibidos no nome de arquivo do Windows.
var nomeDePrograma = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*\.exe$`)

// nomeReservado são os nomes de dispositivo do Windows, que valem com qualquer extensão (`nul.exe`
// abre o dispositivo, e não um arquivo).
var nomeReservado = regexp.MustCompile(`(?i)^(CON|PRN|AUX|NUL|COM[0-9]|LPT[0-9])$`)

// caminhoValido: absoluto; ou, com `-relativo`, só o nome de um `.exe` (`nomeDePrograma`) que não seja
// nome de dispositivo.
func caminhoValido(programa string, relativo bool) bool {
	if !relativo {
		return filepath.IsAbs(programa)
	}
	base, _, _ := strings.Cut(programa, ".")
	return nomeDePrograma.MatchString(programa) && !nomeReservado.MatchString(base)
}

func gerar(saida, programa string, relativo bool) error {
	if saida == "" || !caminhoValido(programa, relativo) {
		return errors.New("informe -saida e o caminho absoluto do -programa (ou, com -relativo, só o nome do .exe)")
	}
	ids := origem.ExtensoesChrome()
	if len(ids) == 0 {
		return errors.New("este build não aceita extensão nenhuma do Chrome e do Edge (os IDs das lojas entram na F7a; para testar, use a tag dev)")
	}
	chromium := manifestoChromium{Name: origem.NomeDoHost, Description: descricao, Path: programa, Type: "stdio"}
	for _, id := range ids {
		chromium.AllowedOrigins = append(chromium.AllowedOrigins, "chrome-extension://"+id+"/")
	}
	firefox := manifestoFirefox{Name: origem.NomeDoHost, Description: descricao, Path: programa, Type: "stdio", AllowedExtensions: []string{origem.ExtensaoFirefox}}
	if err := os.MkdirAll(saida, 0o755); err != nil {
		return err
	}
	for nome, conteudo := range map[string]any{"chromium.json": chromium, "firefox.json": firefox} {
		b, err := json.MarshalIndent(conteudo, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(saida, nome), append(b, '\n'), 0o644); err != nil {
			return err
		}
	}
	return nil
}
