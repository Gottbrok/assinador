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

// O p11-kit como segunda fonte: o registro da pessoa esconde o de `/etc`, que esconde o do pacote;
// o `p11-kit-trust` e o `gnome-keyring` ficam de fora; `enable-in` e `disable-in` valem para este
// programa; nome sem caminho se resolve nas pastas de módulos; o OpenSC registrado é o mesmo arquivo
// do catálogo e não se repete.
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

	pessoa := filepath.Join(dir, "pessoa")
	etc := filepath.Join(dir, "etc")
	share := filepath.Join(dir, "share")
	arquivo(t, filepath.Join(share, "opensc-pkcs11.module"), "# comentário\nmodule: opensc-pkcs11.so\n")
	arquivo(t, filepath.Join(share, "p11-kit-trust.module"), "module: p11-kit-trust.so\ntrust-policy: yes\n")
	arquivo(t, filepath.Join(share, "gnome-keyring.module"), "module: gnome-keyring-pkcs11.so\nenable-in: geary, midori\n")
	arquivo(t, filepath.Join(share, "fabricante.module"), "module: "+fabricanteDoPacote+"\n")
	arquivo(t, filepath.Join(etc, "fabricante.module"), "module: "+fabricanteDoAdmin+"\n")
	arquivo(t, filepath.Join(share, "convidado.module"), "module: "+convidado+"\nenable-in: firefox assinador\n")
	arquivo(t, filepath.Join(pessoa, "proibido.module"), "module: "+proibido+"\ndisable-in: firefox,assinador\n")
	arquivo(t, filepath.Join(share, "relativo.module"), "module: sub/x.so\n")
	arquivo(t, filepath.Join(share, "sumiu.module"), "module: sumiu.so\n")
	arquivo(t, filepath.Join(share, "sem-modulo.module"), "priority: 1\n")

	cat := []catalogo.Modulo{{Nome: "opensc", Rotulo: "OpenSC", Generico: true, Caminhos: map[string][]string{plataforma: {opensc}}}}
	achados, ausentes := Descobrir(OpcoesDeDescoberta{
		Catalogo:                cat,
		PastasDoP11Kit:          []string{pessoa, etc, share},
		PastasDeModulosDoP11Kit: []string{pastaDeModulos},
	})
	var obtidos []string
	for _, m := range achados {
		obtidos = append(obtidos, m.Origem+":"+m.Caminho)
	}
	quer := []string{
		OrigemCatalogo + ":" + opensc,
		OrigemP11Kit + ":" + convidado,
		OrigemP11Kit + ":" + fabricanteDoAdmin,
	}
	if strings.Join(obtidos, "\n") != strings.Join(quer, "\n") {
		t.Fatalf("achados:\n%s\nesperado:\n%s", strings.Join(obtidos, "\n"), strings.Join(quer, "\n"))
	}
	var nomesAusentes []string
	for _, m := range ausentes {
		nomesAusentes = append(nomesAusentes, m.Nome)
	}
	if strings.Join(nomesAusentes, ",") != "relativo,sumiu" {
		t.Fatalf("ausentes: %v", nomesAusentes)
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
	if strings.Join(o.PastasDoP11Kit, ",") != "/tmp/cfg/pkcs11/modules,/etc/pkcs11/modules,/usr/share/p11-kit/modules" {
		t.Fatalf("pastas do p11-kit: %v", o.PastasDoP11Kit)
	}
}
