// Package pkcs11 é o provedor de chaves do Linux e do macOS: acha os módulos PKCS#11, fala com cada
// um num processo FILHO (regra 8 do CLAUDE.md: biblioteca de fabricante que derruba o processo
// derruba só o filho), e entra no token com um `C_Login` próprio, que zera a cópia do PIN (achado
// da F0: o `Login` do `miekg/pkcs11` não zera).
package pkcs11

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Gottbrok/assinador/nativo/internal/catalogo"
)

// Origens de um módulo.
const (
	OrigemCatalogo     = "catalogo"
	OrigemConfiguracao = "configuracao"
)

// Modulo é um módulo PKCS#11 a carregar.
type Modulo struct {
	Caminho  string
	Nome     string
	Rotulo   string
	Generico bool
	Origem   string
}

// OpcoesDeDescoberta dizem onde procurar. `OpcoesPadrao` é o que o programa usa.
type OpcoesDeDescoberta struct {
	Catalogo         []catalogo.Modulo
	PastaDoSistema   string
	ArquivoDoUsuario string
}

// OpcoesPadrao: o catálogo medido; `/etc/confidata-assinador/modulos.d/*.conf`; e
// `~/.config/confidata-assinador/modulos` (um caminho absoluto por linha, `#` comenta). O p11-kit
// entra na F2b como segunda fonte.
func OpcoesPadrao() OpcoesDeDescoberta {
	o := OpcoesDeDescoberta{Catalogo: catalogo.Modulos, PastaDoSistema: "/etc/confidata-assinador/modulos.d"}
	if dir, err := os.UserConfigDir(); err == nil {
		o.ArquivoDoUsuario = filepath.Join(dir, "confidata-assinador", "modulos")
	}
	return o
}

const (
	maximoDeModulos        = 32
	tamanhoMaximoDoArquivo = 64 * 1024
)

// Descobrir devolve os módulos a carregar, na ordem (catálogo, sistema, pessoa), sem repetir o
// mesmo arquivo por outro caminho (dedupe pelo caminho REAL), e os que foram pedidos e não existem
// (para o diagnóstico dizer "ausente").
func Descobrir(o OpcoesDeDescoberta) (achados, ausentes []Modulo) {
	vistos := map[string]bool{}
	acrescentar := func(m Modulo) {
		real, err := filepath.EvalSymlinks(m.Caminho)
		if err != nil {
			ausentes = append(ausentes, m)
			return
		}
		if fi, err := os.Stat(real); err != nil || !fi.Mode().IsRegular() {
			ausentes = append(ausentes, m)
			return
		}
		if vistos[real] || len(achados) >= maximoDeModulos {
			return
		}
		vistos[real] = true
		achados = append(achados, m)
	}

	for _, c := range o.Catalogo {
		caminhos := c.CaminhosAqui()
		achou := false
		for _, caminho := range caminhos {
			if _, err := os.Stat(caminho); err == nil {
				acrescentar(Modulo{Caminho: caminho, Nome: c.Nome, Rotulo: c.Rotulo, Generico: c.Generico, Origem: OrigemCatalogo})
				achou = true
				break
			}
		}
		if !achou && len(caminhos) > 0 {
			ausentes = append(ausentes, Modulo{Caminho: caminhos[0], Nome: c.Nome, Rotulo: c.Rotulo, Generico: c.Generico, Origem: OrigemCatalogo})
		}
	}

	var arquivos []string
	if o.PastaDoSistema != "" {
		if lidos, err := filepath.Glob(filepath.Join(o.PastaDoSistema, "*.conf")); err == nil {
			slices.Sort(lidos)
			arquivos = append(arquivos, lidos...)
		}
	}
	if o.ArquivoDoUsuario != "" {
		arquivos = append(arquivos, o.ArquivoDoUsuario)
	}
	for _, arquivo := range arquivos {
		for _, caminho := range caminhosDoArquivo(arquivo) {
			base := filepath.Base(caminho)
			acrescentar(Modulo{Caminho: caminho, Nome: base, Rotulo: base, Origem: OrigemConfiguracao})
		}
	}
	return achados, ausentes
}

// caminhosDoArquivo lê um arquivo de configuração: um caminho ABSOLUTO por linha; linha vazia e
// linha que começa por `#` não contam; caminho relativo é ignorado (não se carrega biblioteca
// relativa ao diretório de quem lançou o programa).
func caminhosDoArquivo(arquivo string) []string {
	f, err := os.Open(arquivo)
	if err != nil {
		return nil
	}
	defer f.Close()
	var saida []string
	s := bufio.NewScanner(io.LimitReader(f, tamanhoMaximoDoArquivo))
	for s.Scan() {
		linha := strings.TrimSpace(s.Text())
		if linha == "" || strings.HasPrefix(linha, "#") || !filepath.IsAbs(linha) {
			continue
		}
		saida = append(saida, filepath.Clean(linha))
	}
	return saida
}
