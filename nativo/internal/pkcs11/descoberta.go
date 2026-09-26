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
	"runtime"
	"slices"
	"strings"

	"github.com/Gottbrok/assinador/nativo/internal/catalogo"
)

// Origens de um módulo.
const (
	OrigemCatalogo     = "catalogo"
	OrigemP11Kit       = "p11-kit"
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
	Catalogo []catalogo.Modulo
	// PastasDoP11Kit são as pastas de registro do p11-kit (`*.module`), da que MAIS vale para a
	// que menos vale: o registro de mesmo nome numa pasta anterior esconde o das seguintes.
	PastasDoP11Kit []string
	// PastasDeModulosDoP11Kit são onde o p11-kit procura o módulo registrado por NOME (sem
	// caminho), como o `opensc-pkcs11.so`.
	PastasDeModulosDoP11Kit []string
	PastaDoSistema          string
	ArquivoDoUsuario        string
}

// nomeDoPrograma é como o p11-kit chama este programa no `enable-in` e no `disable-in`.
const nomeDoPrograma = "assinador"

// registrosIgnorados não são de cartão: o `p11-kit-trust` é o repositório de ACs do sistema, e o
// `gnome-keyring` guarda senhas.
var registrosIgnorados = []string{"p11-kit-trust", "gnome-keyring", "p11-kit-trust.so", "gnome-keyring-pkcs11.so"}

// pastasDeModulosDoP11Kit: o `$(libdir)/pkcs11` do Debian e do Ubuntu (multiarch, medido na F2a e
// na F2b) e do Fedora (medido na F2b, em contêiner).
var pastasDeModulosDoP11Kit = map[string][]string{
	"linux/amd64": {"/usr/lib/x86_64-linux-gnu/pkcs11", "/usr/lib64/pkcs11"},
	"linux/arm64": {"/usr/lib/aarch64-linux-gnu/pkcs11", "/usr/lib64/pkcs11"},
}

// OpcoesPadrao: o catálogo medido; os registros do p11-kit (`~/.config/pkcs11/modules`,
// `/etc/pkcs11/modules` e `/usr/share/p11-kit/modules`); `/etc/confidata-assinador/modulos.d/*.conf`;
// e `~/.config/confidata-assinador/modulos` (um caminho absoluto por linha, `#` comenta).
func OpcoesPadrao() OpcoesDeDescoberta {
	o := OpcoesDeDescoberta{
		Catalogo:                catalogo.Modulos,
		PastasDoP11Kit:          []string{"/etc/pkcs11/modules", "/usr/share/p11-kit/modules"},
		PastasDeModulosDoP11Kit: pastasDeModulosDoP11Kit[runtime.GOOS+"/"+runtime.GOARCH],
		PastaDoSistema:          "/etc/confidata-assinador/modulos.d",
	}
	if dir, err := os.UserConfigDir(); err == nil {
		o.PastasDoP11Kit = append([]string{filepath.Join(dir, "pkcs11", "modules")}, o.PastasDoP11Kit...)
		o.ArquivoDoUsuario = filepath.Join(dir, "confidata-assinador", "modulos")
	}
	return o
}

const (
	maximoDeModulos        = 32
	tamanhoMaximoDoArquivo = 64 * 1024
)

// Descobrir devolve os módulos a carregar, na ordem (catálogo, p11-kit, sistema, pessoa), sem
// repetir o mesmo arquivo por outro caminho (dedupe pelo caminho REAL: o OpenSC que o p11-kit
// registra é o mesmo arquivo do catálogo), e os que foram pedidos e não existem (para o diagnóstico
// dizer "ausente").
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

	for _, r := range registrosDoP11Kit(o.PastasDoP11Kit) {
		caminho, ok := resolverNoP11Kit(r.modulo, o.PastasDeModulosDoP11Kit)
		if !ok {
			ausentes = append(ausentes, Modulo{Caminho: r.modulo, Nome: r.nome, Rotulo: r.nome, Origem: OrigemP11Kit})
			continue
		}
		acrescentar(Modulo{Caminho: caminho, Nome: r.nome, Rotulo: r.nome, Origem: OrigemP11Kit})
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

// registroDoP11Kit é um arquivo `*.module` do p11-kit, lido.
type registroDoP11Kit struct {
	nome   string
	modulo string
}

// registrosDoP11Kit lê os registros das pastas, na ordem de valor: o registro de mesmo nome numa
// pasta anterior esconde os das seguintes (é a regra do p11-kit: o da pessoa sobre o de `/etc`, e
// este sobre o do pacote). Ficam de fora os que não são de cartão (`registrosIgnorados`), os que
// têm `enable-in` sem este programa e os que têm `disable-in` com ele.
func registrosDoP11Kit(pastas []string) []registroDoP11Kit {
	vistos := map[string]bool{}
	var saida []registroDoP11Kit
	for _, pasta := range pastas {
		arquivos, err := filepath.Glob(filepath.Join(pasta, "*.module"))
		if err != nil {
			continue
		}
		slices.Sort(arquivos)
		for _, arquivo := range arquivos {
			nome := strings.TrimSuffix(filepath.Base(arquivo), ".module")
			if vistos[nome] {
				continue
			}
			vistos[nome] = true
			campos := camposDoRegistro(arquivo)
			modulo := campos["module"]
			if modulo == "" || slices.Contains(registrosIgnorados, nome) || slices.Contains(registrosIgnorados, filepath.Base(modulo)) {
				continue
			}
			if habilitado, ok := campos["enable-in"]; ok && !slices.Contains(listaDoRegistro(habilitado), nomeDoPrograma) {
				continue
			}
			if slices.Contains(listaDoRegistro(campos["disable-in"]), nomeDoPrograma) {
				continue
			}
			saida = append(saida, registroDoP11Kit{nome: nome, modulo: modulo})
		}
	}
	// A ordem é a do nome do registro, e não a da pasta onde ele mora.
	slices.SortFunc(saida, func(a, b registroDoP11Kit) int { return strings.Compare(a.nome, b.nome) })
	return saida
}

// camposDoRegistro lê as linhas `chave: valor` de um `*.module` (`#` comenta).
func camposDoRegistro(arquivo string) map[string]string {
	campos := map[string]string{}
	f, err := os.Open(arquivo)
	if err != nil {
		return campos
	}
	defer f.Close()
	s := bufio.NewScanner(io.LimitReader(f, tamanhoMaximoDoArquivo))
	for s.Scan() {
		linha := strings.TrimSpace(s.Text())
		if linha == "" || strings.HasPrefix(linha, "#") {
			continue
		}
		chave, valor, ok := strings.Cut(linha, ":")
		if !ok {
			continue
		}
		campos[strings.TrimSpace(chave)] = strings.TrimSpace(valor)
	}
	return campos
}

// listaDoRegistro separa um `enable-in` ou `disable-in` (nomes separados por vírgula ou espaço).
func listaDoRegistro(valor string) []string {
	return strings.FieldsFunc(valor, func(r rune) bool { return r == ',' || r == ' ' || r == '\t' })
}

// resolverNoP11Kit acha o arquivo de um módulo registrado: caminho absoluto como está; nome sem
// caminho, nas pastas de módulos do p11-kit. Nome relativo COM pasta (`sub/x.so`) é recusado: não se
// carrega biblioteca por caminho relativo.
func resolverNoP11Kit(modulo string, pastas []string) (string, bool) {
	if filepath.IsAbs(modulo) {
		return filepath.Clean(modulo), true
	}
	if strings.ContainsRune(modulo, '/') || modulo == "." || modulo == ".." {
		return "", false
	}
	for _, pasta := range pastas {
		caminho := filepath.Join(pasta, modulo)
		if _, err := os.Stat(caminho); err == nil {
			return caminho, true
		}
	}
	return "", false
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
