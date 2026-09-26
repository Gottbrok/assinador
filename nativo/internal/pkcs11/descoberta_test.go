//go:build !windows

package pkcs11

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Gottbrok/assinador/nativo/internal/catalogo"
)

func arquivo(t *testing.T, caminho, conteudo string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(caminho), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caminho, []byte(conteudo), 0o600); err != nil {
		t.Fatal(err)
	}
	return caminho
}

func TestDescobrirNaOrdemESemRepetir(t *testing.T) {
	dir := t.TempDir()
	plataforma := runtime.GOOS + "/" + runtime.GOARCH
	fabricante := arquivo(t, filepath.Join(dir, "lib", "fabricante.so"), "x")
	generico := arquivo(t, filepath.Join(dir, "lib", "generico.so"), "x")
	doSistema := arquivo(t, filepath.Join(dir, "lib", "sistema.so"), "x")
	daPessoa := arquivo(t, filepath.Join(dir, "lib", "pessoa.so"), "x")
	atalho := filepath.Join(dir, "lib", "atalho.so")
	if err := os.Symlink(fabricante, atalho); err != nil {
		t.Fatal(err)
	}
	cat := []catalogo.Modulo{
		{Nome: "fabricante", Rotulo: "Fabricante", Caminhos: map[string][]string{plataforma: {filepath.Join(dir, "nao-existe.so"), fabricante}}},
		{Nome: "generico", Rotulo: "Genérico", Generico: true, Caminhos: map[string][]string{plataforma: {generico}}},
		{Nome: "ausente", Rotulo: "Ausente", Caminhos: map[string][]string{plataforma: {filepath.Join(dir, "ausente.so")}}},
		{Nome: "outra-plataforma", Rotulo: "Outra", Caminhos: map[string][]string{"plan9/386": {fabricante}}},
	}
	sistema := filepath.Join(dir, "etc", "modulos.d")
	arquivo(t, filepath.Join(sistema, "b.conf"), "# comentário\n\n"+doSistema+"\n")
	arquivo(t, filepath.Join(sistema, "a.conf"), atalho+"\nrelativo/nao.so\n")
	arquivo(t, filepath.Join(sistema, "ignorado.txt"), daPessoa+"\n")
	usuario := arquivo(t, filepath.Join(dir, "home", "modulos"), "  "+daPessoa+"  \n"+filepath.Join(dir, "sumiu.so")+"\n"+generico+"\n")

	achados, ausentes := Descobrir(OpcoesDeDescoberta{Catalogo: cat, PastaDoSistema: sistema, ArquivoDoUsuario: usuario})
	var caminhos []string
	for _, m := range achados {
		caminhos = append(caminhos, m.Caminho)
	}
	quer := []string{fabricante, generico, doSistema, daPessoa}
	if len(caminhos) != len(quer) {
		t.Fatalf("achados %v", caminhos)
	}
	for i := range quer {
		if caminhos[i] != quer[i] {
			t.Fatalf("achados %v, esperado %v", caminhos, quer)
		}
	}
	if achados[0].Origem != OrigemCatalogo || !achados[1].Generico || achados[2].Origem != OrigemConfiguracao || achados[3].Rotulo != "pessoa.so" {
		t.Fatalf("metadados: %+v", achados)
	}
	var nomesAusentes []string
	for _, m := range ausentes {
		nomesAusentes = append(nomesAusentes, m.Nome)
	}
	if len(ausentes) != 2 || nomesAusentes[0] != "ausente" || nomesAusentes[1] != "sumiu.so" {
		t.Fatalf("ausentes: %v", nomesAusentes)
	}
}

// O p11-kit como segunda fonte, com as regras dele: o registro de mesmo nome se junta campo a campo
// (a pessoa sobre o sistema, o sistema sobre o pacote), e `module:` em branco desliga; o
// `p11-kit-trust` e o `gnome-keyring` ficam de fora pelo nome do registro E pelo da biblioteca;
// `enable-in` e `disable-in` valem para este programa; nome sem caminho se resolve nas pastas de
// módulos; o OpenSC registrado é o mesmo arquivo do catálogo e não se repete.
func TestDescobrirPeloP11Kit(t *testing.T) {
	dir := t.TempDir()
	plataforma := runtime.GOOS + "/" + runtime.GOARCH
	opensc := arquivo(t, filepath.Join(dir, "lib", "opensc-pkcs11.so"), "x")
	pastaDeModulos := filepath.Join(dir, "lib", "pkcs11")
	if err := os.MkdirAll(pastaDeModulos, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(opensc, filepath.Join(pastaDeModulos, "opensc-pkcs11.so")); err != nil {
		t.Fatal(err)
	}
	arquivo(t, filepath.Join(pastaDeModulos, "p11-kit-trust.so"), "x")
	arquivo(t, filepath.Join(pastaDeModulos, "gnome-keyring-pkcs11.so"), "x")
	fabricanteDoPacote := arquivo(t, filepath.Join(dir, "opt", "fab-pacote.so"), "x")
	fabricanteDoAdmin := arquivo(t, filepath.Join(dir, "opt", "fab-admin.so"), "x")
	convidado := arquivo(t, filepath.Join(dir, "opt", "convidado.so"), "x")
	proibido := arquivo(t, filepath.Join(dir, "opt", "proibido.so"), "x")
	liberado := arquivo(t, filepath.Join(dir, "opt", "liberado.so"), "x")
	desligado := arquivo(t, filepath.Join(dir, "opt", "desligado.so"), "x")

	pessoa := filepath.Join(dir, "pessoa")
	etc := filepath.Join(dir, "etc")
	share := filepath.Join(dir, "share")
	arquivo(t, filepath.Join(share, "opensc-pkcs11.module"), "# comentário\nmodule: opensc-pkcs11.so\n")
	arquivo(t, filepath.Join(share, "p11-kit-trust.module"), "module: p11-kit-trust.so\ntrust-policy: yes\n")
	// Sem `enable-in`: só a regra do nome o tira; e um registro de outro nome para a mesma biblioteca.
	arquivo(t, filepath.Join(share, "gnome-keyring.module"), "module: gnome-keyring-pkcs11.so\n")
	arquivo(t, filepath.Join(share, "chaveiro.module"), "module: gnome-keyring-pkcs11.so\n")
	arquivo(t, filepath.Join(share, "fabricante.module"), "module: "+fabricanteDoPacote+"\n")
	arquivo(t, filepath.Join(etc, "fabricante.module"), "module: "+fabricanteDoAdmin+"\n")
	arquivo(t, filepath.Join(share, "convidado.module"), "module: "+convidado+"\nenable-in: firefox assinador\n")
	arquivo(t, filepath.Join(pessoa, "proibido.module"), "module: "+proibido+"\ndisable-in: firefox,assinador\n")
	// O sistema só habilita no Firefox; a pessoa acrescenta este programa, e o `module:` vem do sistema.
	arquivo(t, filepath.Join(etc, "liberado.module"), "module: "+liberado+"\nenable-in: firefox\n")
	arquivo(t, filepath.Join(pessoa, "liberado.module"), "enable-in: assinador\n")
	// O pacote registra; a pessoa desliga com `module:` em branco.
	arquivo(t, filepath.Join(share, "desligado.module"), "module: "+desligado+"\n")
	arquivo(t, filepath.Join(pessoa, "desligado.module"), "module:\n")
	arquivo(t, filepath.Join(share, "relativo.module"), "module: sub/x.so\n")
	arquivo(t, filepath.Join(share, "sumiu.module"), "module: sumiu.so\n")
	arquivo(t, filepath.Join(share, "sem-modulo.module"), "priority: 1\n")

	cat := []catalogo.Modulo{{Nome: "opensc", Rotulo: "OpenSC", Generico: true, Caminhos: map[string][]string{plataforma: {opensc}}}}
	opcoes := OpcoesDeDescoberta{
		Catalogo:                cat,
		PastaDoP11KitDoPacote:   share,
		PastaDoP11KitDoSistema:  etc,
		PastaDoP11KitDaPessoa:   pessoa,
		PastasDeModulosDoP11Kit: []string{pastaDeModulos},
	}
	caminhosDe := func(achados []Modulo) string {
		var obtidos []string
		for _, m := range achados {
			obtidos = append(obtidos, m.Origem+":"+m.Caminho)
		}
		return strings.Join(obtidos, "\n")
	}
	achados, ausentes := Descobrir(opcoes)
	quer := strings.Join([]string{
		OrigemCatalogo + ":" + opensc,
		OrigemP11Kit + ":" + convidado,
		OrigemP11Kit + ":" + fabricanteDoAdmin,
		OrigemP11Kit + ":" + liberado,
	}, "\n")
	if caminhosDe(achados) != quer {
		t.Fatalf("achados:\n%s\nesperado:\n%s", caminhosDe(achados), quer)
	}
	var nomesAusentes []string
	for _, m := range ausentes {
		nomesAusentes = append(nomesAusentes, m.Nome)
	}
	if strings.Join(nomesAusentes, ",") != "relativo,sumiu" {
		t.Fatalf("ausentes: %v", nomesAusentes)
	}

	// `user-config: none` no sistema: a pasta da pessoa não conta (o `proibido` volta, o
	// `liberado` sai, o `desligado` volta a valer). `only`: só a da pessoa conta.
	opcoes.ConfiguracaoDoP11Kit = arquivo(t, filepath.Join(dir, "etc-global", "pkcs11.conf"), "# global\nuser-config: none\n")
	achados, _ = Descobrir(opcoes)
	quer = strings.Join([]string{
		OrigemCatalogo + ":" + opensc,
		OrigemP11Kit + ":" + convidado,
		OrigemP11Kit + ":" + desligado,
		OrigemP11Kit + ":" + fabricanteDoAdmin,
	}, "\n")
	if caminhosDe(achados) != quer {
		t.Fatalf("user-config none:\n%s", caminhosDe(achados))
	}
	arquivo(t, opcoes.ConfiguracaoDoP11Kit, "user-config: only\n")
	achados, _ = Descobrir(opcoes)
	if caminhosDe(achados) != OrigemCatalogo+":"+opensc {
		t.Fatalf("user-config only:\n%s", caminhosDe(achados))
	}
}

// O módulo achado pelo p11-kit ou pela configuração com o nome de arquivo de um do catálogo é
// aquele módulo: o OpenSC registrado fora do caminho medido continua genérico, e o SafeSign num
// caminho que o catálogo não mediu continua sendo o SafeSign.
func TestModuloForaDoCaminhoMedidoEhOCatalogo(t *testing.T) {
	dir := t.TempDir()
	opensc := arquivo(t, filepath.Join(dir, "pkcs11", "opensc-pkcs11.so"), "x")
	safesign := arquivo(t, filepath.Join(dir, "lib64", "libaetpkss.so.3"), "x")
	arquivo(t, filepath.Join(dir, "share", "opensc.module"), "module: opensc-pkcs11.so\n")
	usuario := arquivo(t, filepath.Join(dir, "home", "modulos"), safesign+"\n")
	achados, _ := Descobrir(OpcoesDeDescoberta{
		Catalogo:                catalogo.Modulos,
		PastaDoP11KitDoPacote:   filepath.Join(dir, "share"),
		PastasDeModulosDoP11Kit: []string{filepath.Join(dir, "pkcs11")},
		ArquivoDoUsuario:        usuario,
	})
	var vistos []string
	for _, m := range achados {
		if m.Caminho == opensc || m.Caminho == safesign {
			vistos = append(vistos, m.Nome+"/"+m.Rotulo+"/"+m.Origem)
			if (m.Nome == "opensc") != m.Generico {
				t.Errorf("%s: genérico = %v", m.Nome, m.Generico)
			}
		}
	}
	if strings.Join(vistos, ",") != "opensc/OpenSC/"+OrigemP11Kit+",safesign/SafeSign/"+OrigemConfiguracao {
		t.Fatalf("identificados: %v", vistos)
	}
}

func TestOpcoesPadraoUsamOCatalogoMedido(t *testing.T) {
	o := OpcoesPadrao()
	if len(o.Catalogo) != len(catalogo.Modulos) || o.PastaDoSistema != "/etc/confidata-assinador/modulos.d" {
		t.Fatalf("%+v", o)
	}
	t.Setenv("XDG_CONFIG_HOME", "/tmp/cfg")
	o = OpcoesPadrao()
	if o.ArquivoDoUsuario != "/tmp/cfg/confidata-assinador/modulos" {
		t.Fatalf("arquivo da pessoa: %s", o.ArquivoDoUsuario)
	}
	if o.PastaDoP11KitDoPacote != "/usr/share/p11-kit/modules" || o.PastaDoP11KitDoSistema != "/etc/pkcs11/modules" || o.PastaDoP11KitDaPessoa != "/tmp/cfg/pkcs11/modules" || o.ConfiguracaoDoP11Kit != "/etc/pkcs11/pkcs11.conf" {
		t.Fatalf("pastas do p11-kit: %+v", o)
	}
}
