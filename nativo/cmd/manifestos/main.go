// Comando manifestos: escreve os manifestos de native messaging dos navegadores, que o pacote
// instala.
//
//	go run -tags dev ./cmd/manifestos -saida <pasta> -programa /usr/lib/confidata-assinador/assinador
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

	"github.com/Gottbrok/assinador/nativo/internal/origem"
)

const descricao = "Assinador: assina com o certificado digital do cartão ou do token"

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
	programa := flag.String("programa", "", "caminho ABSOLUTO do programa instalado")
	flag.Parse()
	if err := gerar(*saida, *programa); err != nil {
		fmt.Fprintln(os.Stderr, "manifestos:", err)
		os.Exit(1)
	}
}

func gerar(saida, programa string) error {
	if saida == "" || !filepath.IsAbs(programa) {
		return errors.New("informe -saida e o caminho absoluto do -programa")
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
