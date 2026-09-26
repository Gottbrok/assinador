//go:build windows

package windows

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"slices"
	"testing"
	"unsafe"

	win "golang.org/x/sys/windows"

	"github.com/Gottbrok/assinador/nativo/internal/assinatura"
	"github.com/Gottbrok/assinador/nativo/internal/diagnostico"
	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
	"github.com/Gottbrok/assinador/nativo/internal/windowsteste"
)

var digestDoTeste = sha256.Sum256([]byte("documento do teste do provedor do Windows"))

// listado acha, na lista do provedor, o certificado de DER `der`.
func listado(t *testing.T, p *Provedor, der []byte) assinatura.Certificado {
	t.Helper()
	certs, avisos := p.Listar(context.Background())
	if len(avisos) > 0 {
		t.Fatalf("avisos: %v", avisos)
	}
	for _, c := range certs {
		if bytes.Equal(c.DER, der) {
			return c
		}
	}
	t.Fatalf("o certificado criado não veio na lista (%d certificados)", len(certs))
	return assinatura.Certificado{}
}

func conferir(t *testing.T, der []byte, assinada []byte) {
	t.Helper()
	x, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	if err := assinatura.ConferirAssinatura(x, digestDoTeste, assinada); err != nil {
		t.Fatalf("a assinatura não confere com o certificado: %v", err)
	}
}

// comContexto chama `f` com o contexto do certificado de DER `der` no repositório pessoal.
func comContexto(t *testing.T, der []byte, f func(ctx *win.CertContext)) {
	t.Helper()
	p := NovoProvedor(0)
	repositorio, err := p.abrirRepositorio()
	if err != nil {
		t.Fatal(err)
	}
	defer win.CertCloseStore(repositorio, 0)
	ctx := percorrer(repositorio, func(_ *win.CertContext, v visto) bool { return !bytes.Equal(v.der, der) })
	if ctx == nil {
		t.Fatal("o certificado criado não está no repositório")
	}
	defer win.CertFreeCertificateContext(ctx)
	f(ctx)
}

// A chave CNG (KSP): listada sem PIN, com o rótulo de chave instalada no Windows, e assinando; a
// assinatura confere com o certificado. Com uma janela-mãe de verdade, o caminho da janela também.
func TestListarEAssinarPeloCNG(t *testing.T) {
	c := windowsteste.Novo(t, windowsteste.KSP)
	p := NovoProvedor(0)
	cert := listado(t, p, c.DER)
	if cert.Ref != assinatura.Ref(c.DER) || cert.Provedor != "windows:"+CaminhoCNG || cert.RotuloDoProvedor != RotuloInstalado || cert.ExigePin {
		t.Fatalf("certificado listado: %+v", cert)
	}
	assinada, err := p.Assinar(context.Background(), cert, digestDoTeste, nil)
	if err != nil {
		t.Fatal(err)
	}
	conferir(t, c.DER, assinada)

	janela, _, _ := win.NewLazySystemDLL("user32.dll").NewProc("GetDesktopWindow").Call()
	comJanela := NovoProvedor(uint64(janela))
	assinada, err = comJanela.Assinar(context.Background(), cert, digestDoTeste, nil)
	if err != nil {
		t.Fatalf("com a janela-mãe: %v", err)
	}
	conferir(t, c.DER, assinada)
}

// A chave do CSP legado, pelo caminho do CSP (o do cartão com CSP do fabricante): o `CryptSignHash`
// devolve em little-endian, e só a inversão faz a assinatura conferir. Pelo caminho do programa (que
// prefere o CNG, e o CNG também abre os CSP da Microsoft), a assinatura confere do mesmo jeito.
func TestAssinarPeloCSPComInversao(t *testing.T) {
	c := windowsteste.Novo(t, windowsteste.CSP)
	p := NovoProvedor(0)
	if cert := listado(t, p, c.DER); cert.Provedor != "windows:"+CaminhoCSP || cert.RotuloDoProvedor != RotuloInstalado {
		t.Fatalf("certificado listado: %+v", cert)
	}
	comContexto(t, c.DER, func(ctx *win.CertContext) {
		assinada, caminho, e := assinarComContexto(ctx, digestDoTeste, false, 0)
		if e != nil {
			t.Fatal(e)
		}
		if caminho != CaminhoCSP {
			t.Fatalf("o teste do CSP passou pelo caminho %s", caminho)
		}
		conferir(t, c.DER, assinada)
		// Sem a inversão, o que o CSP devolveu não confere: a inversão é o que faz o caminho funcionar.
		invertida := bytes.Clone(assinada)
		inverter(invertida)
		x, _ := x509.ParseCertificate(c.DER)
		if assinatura.ConferirAssinatura(x, digestDoTeste, invertida) == nil {
			t.Fatal("a assinatura na ordem do CSP também conferiu: o teste não distingue a inversão")
		}

		assinada, _, e = assinarComContexto(ctx, digestDoTeste, true, 0)
		if e != nil {
			t.Fatal(e)
		}
		conferir(t, c.DER, assinada)
	})
}

// O CSP legado sem SHA-256 recusa o resumo: `algoritmo-nao-suportado`, e não `modulo-falhou`. No CI
// (`ASSINADOR_EXIGE_WINDOWS=1`) ele é obrigatório, como os outros: pulado, o caminho do
// NTE_BAD_ALGID nunca rodaria.
func TestCSPSemSHA256(t *testing.T) {
	c := windowsteste.Novo(t, windowsteste.CSPSemSHA256)
	comContexto(t, c.DER, func(ctx *win.CertContext) {
		_, caminho, e := assinarComContexto(ctx, digestDoTeste, false, 0)
		if caminho != CaminhoCSP || e == nil || e.Codigo != protocolo.AlgoritmoNaoSuportado {
			t.Fatalf("caminho %s, erro %v", caminho, e)
		}
	})
}

// O certificado que saiu do repositório entre a lista e a assinatura não assina.
func TestCertificadoQueSaiuDoRepositorio(t *testing.T) {
	c := windowsteste.Novo(t, windowsteste.KSP)
	outro := bytes.Clone(c.DER)
	outro[len(outro)-1] ^= 0xFF
	_, err := NovoProvedor(0).Assinar(context.Background(), assinatura.Certificado{DER: outro}, digestDoTeste, nil)
	var e *protocolo.Erro
	if err == nil || !errors.As(err, &e) || e.Codigo != protocolo.CertificadoNaoEncontrado {
		t.Fatalf("%v", err)
	}
}

// O diagnóstico conta os certificados com chave por provedor, com o nome que o Windows dá a ele.
func TestDiagnosticar(t *testing.T) {
	c := windowsteste.Novo(t, windowsteste.KSP)
	relatorios := NovoProvedor(0).Diagnosticar(context.Background())
	i := slices.IndexFunc(relatorios, func(r assinatura.RelatorioDoProvedor) bool { return r.Nome == windowsteste.KSP })
	if i < 0 {
		t.Fatalf("o provedor do certificado criado não apareceu: %+v", relatorios)
	}
	r := relatorios[i]
	if r.Estado != assinatura.EstadoCarregado || r.Detalhe != CaminhoCNG || r.Certificados != len(r.Vistos) ||
		!slices.ContainsFunc(r.Vistos, func(v assinatura.Certificado) bool { return bytes.Equal(v.DER, c.DER) }) {
		t.Fatalf("%+v", r)
	}
}

// O estado do serviço de Propagação de Certificados sai sempre num dos três valores.
func TestEstadoDoServicoDePropagacao(t *testing.T) {
	e := EstadoDoServicoDePropagacao()
	if e != diagnostico.ServicoRodando && e != diagnostico.ServicoParado && e != diagnostico.ServicoDesconhecido {
		t.Fatalf("%q", e)
	}
	t.Logf("CertPropSvc: %s", e)
}

// O CRYPT_KEY_PROV_INFO tem o tamanho que o Windows declara para ele (o teste de cabeçalho confere os
// deslocamentos; este, que a estrutura cabe no que a propriedade devolve).
func TestProvedorDaChaveLeOCertificadoCriado(t *testing.T) {
	c := windowsteste.Novo(t, windowsteste.KSP)
	comContexto(t, c.DER, func(ctx *win.CertContext) {
		nome, tipo, ok := provedorDaChave(ctx)
		if !ok || nome != windowsteste.KSP || tipo != 0 {
			t.Fatalf("%q %d %v (a estrutura tem %d bytes)", nome, tipo, ok, unsafe.Sizeof(infoDoProvedorDaChave{}))
		}
	})
}
