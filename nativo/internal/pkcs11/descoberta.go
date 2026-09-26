// Package pkcs11 é o provedor de chaves do Linux e do macOS: acha os módulos PKCS#11, fala com cada
// um num processo FILHO (regra 8 do CLAUDE.md: biblioteca de fabricante que derruba o processo
// derruba só o filho), e entra no token com um `C_Login` próprio, que zera a cópia do PIN (achado
// da F0: o `Login` do `miekg/pkcs11` não zera).
package pkcs11

import (
	"bufio"
	"io"
	"maps"
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
	// As pastas de registro do p11-kit (`*.module`): a do pacote, a do sistema e a da pessoa. Como
	// no p11-kit (`pkcs11.conf(5)`), o registro de mesmo nome se JUNTA campo a campo, e o da pessoa
	// vale sobre o do sistema, que vale sobre o do pacote; `module:` em branco desliga o módulo.
	PastaDoP11KitDoPacote  string
	PastaDoP11KitDoSistema string
	PastaDoP11KitDaPessoa  string
	// ConfiguracaoDoP11Kit é o `pkcs11.conf` do sistema, cujo `user-config` (`none`, `merge`, que é
	// o padrão, ou `only`) diz se a pasta da pessoa conta.
	ConfiguracaoDoP11Kit string
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

// pastasDeModulosDoP11Kit: o `$(libdir)/pkcs11`. Medidos: o do Ubuntu x86_64 (na F2a e na F2b) e o
// do Fedora x86_64 (na F2b, em contêiner). O do arm64 segue a convenção multiarch do Debian e o
// `lib64` do Fedora, e não foi medido.
var pastasDeModulosDoP11Kit = map[string][]string{
	"linux/amd64": {"/usr/lib/x86_64-linux-gnu/pkcs11", "/usr/lib64/pkcs11"},
	"linux/arm64": {"/usr/lib/aarch64-linux-gnu/pkcs11", "/usr/lib64/pkcs11"},
}

// OpcoesPadrao: o catálogo medido; os registros do p11-kit (`/usr/share/p11-kit/modules`,
// `/etc/pkcs11/modules` e `~/.config/pkcs11/modules`, com o `user-config` de
// `/etc/pkcs11/pkcs11.conf`); `/etc/confidata-assinador/modulos.d/*.conf`; e
// `~/.config/confidata-assinador/modulos` (um caminho absoluto por linha, `#` comenta).
func OpcoesPadrao() OpcoesDeDescoberta {
	o := OpcoesDeDescoberta{
		Catalogo:                catalogo.Modulos,
		PastaDoP11KitDoPacote:   "/usr/share/p11-kit/modules",
		PastaDoP11KitDoSistema:  "/etc/pkcs11/modules",
		ConfiguracaoDoP11Kit:    "/etc/pkcs11/pkcs11.conf",
		PastasDeModulosDoP11Kit: pastasDeModulosDoP11Kit[runtime.GOOS+"/"+runtime.GOARCH],
		PastaDoSistema:          "/etc/confidata-assinador/modulos.d",
	}
	if dir, err := os.UserConfigDir(); err == nil {
		o.PastaDoP11KitDaPessoa = filepath.Join(dir, "pkcs11", "modules")
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

	// Módulo achado pelo p11-kit ou pela configuração cuja biblioteca tem o nome de uma do catálogo
	// é aquele módulo (o rótulo e o `Generico` do catálogo): o OpenSC registrado continua genérico
	// na fusão, e o SafeSign fora do caminho medido continua sendo o SafeSign no diagnóstico.
	identificar := func(caminho, nome, origem string) Modulo {
		if c, ok := catalogo.PeloArquivo(caminho, o.Catalogo); ok {
			return Modulo{Caminho: caminho, Nome: c.Nome, Rotulo: c.Rotulo, Generico: c.Generico, Origem: origem}
		}
		return Modulo{Caminho: caminho, Nome: nome, Rotulo: nome, Origem: origem}
	}

	for _, r := range registrosDoP11Kit(o) {
		caminho, ok := resolverNoP11Kit(r.modulo, o.PastasDeModulosDoP11Kit)
		if !ok {
			ausentes = append(ausentes, identificar(r.modulo, r.nome, OrigemP11Kit))
			continue
		}
		acrescentar(identificar(caminho, r.nome, OrigemP11Kit))
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
			acrescentar(identificar(caminho, filepath.Base(caminho), OrigemConfiguracao))
		}
	}
	return achados, ausentes
}

// registroDoP11Kit é um arquivo `*.module` do p11-kit, lido.
type registroDoP11Kit struct {
	nome   string
	modulo string
}

// registrosDoP11Kit lê os registros como o p11-kit (`pkcs11.conf(5)`): o de mesmo nome se JUNTA
// campo a campo, com o do sistema sobre o do pacote e o da pessoa sobre os dois; a pasta da pessoa
// só conta se o `user-config` do sistema não for `none` (e, com `only`, só ela conta); `module:` em
// branco desliga o registro. Ficam de fora os que não são de cartão (`registrosIgnorados`), os que
// têm `enable-in` sem este programa e os que têm `disable-in` com ele. A ordem é a do nome.
func registrosDoP11Kit(o OpcoesDeDescoberta) []registroDoP11Kit {
	var pastas []string
	switch configuracaoDaPessoa(o.ConfiguracaoDoP11Kit) {
	case "none":
		pastas = []string{o.PastaDoP11KitDoPacote, o.PastaDoP11KitDoSistema}
	case "only":
		pastas = []string{o.PastaDoP11KitDaPessoa}
	default:
		pastas = []string{o.PastaDoP11KitDoPacote, o.PastaDoP11KitDoSistema, o.PastaDoP11KitDaPessoa}
	}
	juntos := map[string]map[string]string{}
	for _, pasta := range pastas {
		if pasta == "" {
			continue
		}
		arquivos, err := filepath.Glob(filepath.Join(pasta, "*.module"))
		if err != nil {
			continue
		}
		for _, arquivo := range arquivos {
			nome := strings.TrimSuffix(filepath.Base(arquivo), ".module")
			if juntos[nome] == nil {
				juntos[nome] = map[string]string{}
			}
			for chave, valor := range camposDoRegistro(arquivo) {
				juntos[nome][chave] = valor
			}
		}
	}
	var saida []registroDoP11Kit
	for _, nome := range slices.Sorted(maps.Keys(juntos)) {
		campos := juntos[nome]
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
	return saida
}

// configuracaoDaPessoa é o `user-config` do `pkcs11.conf` do sistema: `none`, `merge` (o padrão)
// ou `only`.
func configuracaoDaPessoa(arquivo string) string {
	if arquivo == "" {
		return "merge"
	}
	switch valor := camposDoRegistro(arquivo)["user-config"]; valor {
	case "none", "only":
		return valor
	}
	return "merge"
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
