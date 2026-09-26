package diagnostico

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"math/big"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Gottbrok/assinador/nativo/internal/assinatura"
	"github.com/Gottbrok/assinador/nativo/internal/catalogo"
	"github.com/Gottbrok/assinador/nativo/internal/pcsc"
)

var agora = time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)

const atrDoTeste = "3BDB9600801F030031C064B0F310000F90"

func certificadoDeTeste(t *testing.T, cn string, publica any, ca bool, ate time.Time) []byte {
	t.Helper()
	ac, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	modelo := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: cn}, Issuer: pkix.Name{CommonName: "AC DE TESTE v5"},
		NotBefore: agora.Add(-time.Hour * 24 * 30), NotAfter: ate,
		KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageContentCommitment, BasicConstraintsValid: true, IsCA: ca,
	}
	pai := &x509.Certificate{Subject: pkix.Name{CommonName: "AC DE TESTE v5"}}
	if cn == "AUTOASSINADO:98765432100" {
		pai = modelo
	}
	der, err := x509.CreateCertificate(rand.Reader, modelo, pai, publica, ac)
	if err != nil {
		t.Fatal(err)
	}
	return der
}

type cena struct {
	fontes     Fontes
	leitoras   pcsc.Resultado
	provedores []assinatura.RelatorioDoProvedor
}

// novaCena é uma cena no LINUX, qualquer que seja o sistema que roda o teste: as frases mudam com o
// sistema, e o teste de cada sistema pede o dele (`novaCenaNo`).
func novaCena(t *testing.T) *cena {
	t.Helper()
	comoSistema(t, "linux")
	chave, _ := rsa.GenerateKey(rand.Reader, 2048)
	der := certificadoDeTeste(t, "TITULAR DE TESTE:12345678901", &chave.PublicKey, false, agora.AddDate(1, 0, 0))
	return &cena{
		fontes: Fontes{
			Agora: func() time.Time { return agora }, Versao: "0.1.0-teste", Plataforma: "linux-amd64", Sistema: "Ubuntu 24.04 LTS",
			Modulos: []catalogo.Modulo{{Nome: "safesign", Rotulo: "SafeSign"}},
			ATRs:    []catalogo.ATR{{Valor: atrDoTeste, Cartao: "cartão de teste", Modulo: "safesign"}},
		},
		leitoras: pcsc.Resultado{Estado: pcsc.EstadoOk, Leitoras: []pcsc.Leitora{{Nome: "Leitora USB 00 00", ComCartao: true, ATR: atrDoTeste}}},
		provedores: []assinatura.RelatorioDoProvedor{
			{Nome: "SafeSign", Caminho: "/usr/lib/libaetpkss.so.3", Origem: "catalogo", Estado: assinatura.EstadoCarregado, Certificados: 1, Detalhe: "A.E.T. Europe B.V., Cryptoki 2.20, biblioteca 3.0",
				Vistos: []assinatura.Certificado{{Ref: assinatura.Ref(der), DER: der, RotuloDoProvedor: "SafeSign", Leitor: "Leitora USB 00 00"}}},
		},
	}
}

func (c *cena) montar() (Relatorio, string) {
	return Montar(c.fontes, c.leitoras, c.provedores, estadoDaPropagacao(c.fontes))
}

var onzeDigitos = regexp.MustCompile(`\d{11}`)

// O relatório e o texto nunca levam o CPF do CN (`NOME:CPF`): os dígitos saem trocados por `*`.
func TestRelatorioSemCpf(t *testing.T) {
	c := novaCena(t)
	chave, _ := rsa.GenerateKey(rand.Reader, 2048)
	auto := certificadoDeTeste(t, "AUTOASSINADO:98765432100", &chave.PublicKey, false, agora.AddDate(1, 0, 0))
	c.provedores[0].Vistos = append(c.provedores[0].Vistos, assinatura.Certificado{Ref: assinatura.Ref(auto), DER: auto, RotuloDoProvedor: "SafeSign"})
	r, texto := c.montar()
	j, _ := json.Marshal(r)
	for _, saida := range []string{string(j), texto} {
		if onzeDigitos.MatchString(saida) || strings.Contains(saida, "12345678901") || strings.Contains(saida, "98765432100") {
			t.Fatalf("o CPF vazou:\n%s", saida)
		}
	}
	if r.Certificados[0].Titular != "TITULAR DE TESTE:***********" || r.Certificados[0].Emissor != "AC DE TESTE v5" {
		t.Fatalf("titular: %+v", r.Certificados[0])
	}
	if r.Certificados[1].Emissor != "AUTOASSINADO:***********" {
		t.Fatalf("o emissor do autoassinado é o próprio titular, e leva o documento: %+v", r.Certificados[1])
	}
	if !strings.Contains(texto, "Leitora Leitora USB 00 00: com cartão, ATR "+atrDoTeste+", cartão de teste (usa o SafeSign)") {
		t.Fatalf("texto:\n%s", texto)
	}
	if len(r.Avisos) != 0 {
		t.Fatalf("tudo certo e mesmo assim avisou: %v", r.Avisos)
	}
}

// O pcscd parado vira frase, e a frase diz o que fazer.
func TestPcscdParadoViraAviso(t *testing.T) {
	c := novaCena(t)
	c.leitoras = pcsc.Resultado{Estado: pcsc.EstadoSemServico, Detalhe: "0x8010001D", Leitoras: []pcsc.Leitora{}}
	r, texto := c.montar()
	if !strings.Contains(strings.Join(r.Avisos, "\n"), "O serviço pcscd não está rodando") || !strings.Contains(texto, "sudo systemctl start pcscd") || !strings.Contains(texto, "PC/SC: serviço pcscd parado") {
		t.Fatalf("avisos: %v\n%s", r.Avisos, texto)
	}
	c.leitoras = pcsc.Resultado{Estado: pcsc.EstadoSemBiblioteca, Leitoras: []pcsc.Leitora{}}
	if r, _ := c.montar(); !strings.Contains(strings.Join(r.Avisos, "\n"), "Instale o pacote pcscd") {
		t.Fatalf("sem biblioteca: %v", r.Avisos)
	}
}

// ATR do catálogo medido sugere o programa do fabricante: sem ele instalado, o aviso manda
// instalar; com ele instalado e sem certificado, o aviso diz isso.
func TestATRConhecidoSugereOMiddleware(t *testing.T) {
	c := novaCena(t)
	c.provedores = []assinatura.RelatorioDoProvedor{{Nome: "OpenSC", Estado: assinatura.EstadoCarregado}, {Nome: "SafeSign", Caminho: "/usr/lib/libaetpkss.so.3", Estado: assinatura.EstadoAusente}}
	r, _ := c.montar()
	avisos := strings.Join(r.Avisos, "\n")
	if r.Leitoras[0].Sugestao != "SafeSign" || !strings.Contains(avisos, "usa o SafeSign, que não está instalado. Instale o SafeSign.") {
		t.Fatalf("sem o SafeSign: %+v %v", r.Leitoras, r.Avisos)
	}
	c.provedores = []assinatura.RelatorioDoProvedor{{Nome: "SafeSign", Estado: assinatura.EstadoCarregado}}
	if r, _ := c.montar(); !strings.Contains(strings.Join(r.Avisos, "\n"), "O SafeSign está instalado, mas não achou certificado no cartão da leitora Leitora USB 00 00.") {
		t.Fatalf("com o SafeSign sem certificado: %v", r.Avisos)
	}
}

// O SafeSign fora do caminho medido (achado pela configuração da pessoa, com o nome do arquivo) é
// reconhecido pelo fabricante que declara; e o certificado que o OpenSC listou antes dele continua
// sendo do SafeSign na contagem do SafeSign. Antes, os dois casos davam aviso falso.
func TestATRConheceOModuloPeloFabricanteEPelaContagemPropria(t *testing.T) {
	c := novaCena(t)
	c.fontes.Modulos = []catalogo.Modulo{{Nome: "safesign", Rotulo: "SafeSign", Fabricante: "A.E.T. Europe B.V."}, {Nome: "opensc", Rotulo: "OpenSC", Fabricante: "OpenSC Project", Generico: true}}
	vistos := c.provedores[0].Vistos
	c.provedores = []assinatura.RelatorioDoProvedor{
		{Nome: "OpenSC", Fabricante: "OpenSC Project", Estado: assinatura.EstadoCarregado, Vistos: vistos},
		{Nome: "libaetpkss.so.3", Origem: "configuracao", Fabricante: "A.E.T. Europe B.V.", Estado: assinatura.EstadoCarregado, Vistos: vistos},
	}
	r, _ := c.montar()
	if len(r.Avisos) != 0 {
		t.Fatalf("aviso falso: %v", r.Avisos)
	}
	if len(r.Certificados) != 1 || r.Provedores[0].Certificados != 1 || r.Provedores[1].Certificados != 1 {
		t.Fatalf("certificados %d, contagens %d e %d", len(r.Certificados), r.Provedores[0].Certificados, r.Provedores[1].Certificados)
	}
	// Sem o SafeSign (só o OpenSC leu), o aviso é o do SafeSign não instalado.
	c.provedores = c.provedores[:1]
	if r, _ := c.montar(); !strings.Contains(strings.Join(r.Avisos, "\n"), "usa o SafeSign, que não está instalado") {
		t.Fatalf("%v", r.Avisos)
	}
}

// O que vem do aparelho e do certificado chega ao texto sem controle: um nome de leitora com
// sequência de escape ou quebra de linha não forja linha nem mexe no terminal do suporte. O emissor
// forjado com documento sai mascarado.
func TestTextoDeForaSaiLimpo(t *testing.T) {
	c := novaCena(t)
	// O RLO (que inverte o texto) montado pelo código, para não ficar invisível no arquivo.
	rlo := string(rune(0x202e))
	c.leitoras.Leitoras[0].Nome = "Leitora\x1b[2J\nAvisos:\n- falso" + rlo + " 00 00"
	c.provedores[0].Detalhe = "fabricante\x07\r\nlinha forjada"
	r, texto := c.montar()
	for _, s := range []string{texto, r.Leitoras[0].Nome, r.Provedores[0].Detalhe} {
		if strings.ContainsAny(s, "\x1b\x07\r"+rlo) {
			t.Fatalf("controle passou: %q", s)
		}
	}
	if strings.Count(texto, "\n- ") != len(r.Avisos) {
		t.Fatalf("o texto tem %d linhas de aviso e o relatório %d avisos:\n%s", strings.Count(texto, "\n- "), len(r.Avisos), texto)
	}
	if strings.Contains(texto, "\n- falso") {
		t.Fatalf("o aviso forjado virou linha:\n%s", texto)
	}
	if got := mascararDocumentos("AC FORJADA 12345678901 v5 e 12.345"); got != "AC FORJADA *********** v5 e 12.345" {
		t.Fatalf("emissor: %q", got)
	}
	if got := limpar(strings.Repeat("x", 300)); len(got) != tamanhoMaximoDoTexto {
		t.Fatalf("sem teto: %d", len(got))
	}
}

// comoSistema faz o `Montar` falar como se rodasse em `goos`.
func comoSistema(t *testing.T, goos string) {
	t.Helper()
	anterior := sistemaOperacional
	sistemaOperacional = goos
	t.Cleanup(func() { sistemaOperacional = anterior })
}

// novaCenaNo é a cena num sistema: a `novaCena` é sempre no Linux, para a suíte dizer o mesmo em
// qualquer executor (o CI roda também no Windows).
func novaCenaNo(t *testing.T, goos string) *cena {
	t.Helper()
	c := novaCena(t)
	comoSistema(t, goos)
	return c
}

// Fora do Linux, a frase não manda instalar o pcscd; no sistema que ainda não lê leitoras (o
// macOS, até a F8), ela diz isso.
func TestAvisoDoPCSCForaDoLinux(t *testing.T) {
	c := novaCenaNo(t, "darwin")
	c.leitoras = pcsc.Resultado{Estado: pcsc.EstadoSemBiblioteca, Leitoras: []pcsc.Leitora{}}
	r, _ := c.montar()
	avisos := strings.Join(r.Avisos, "\n")
	if strings.Contains(avisos, "pcscd") || !strings.Contains(avisos, "ainda não lê as leitoras de cartão neste sistema") {
		t.Fatalf("%v", r.Avisos)
	}
}

// No Windows, as frases do PC/SC falam do Windows: o winscard que não abriu, e o serviço Cartão
// Inteligente, que o Windows só mantém rodando com uma leitora conectada.
func TestAvisosDoPCSCNoWindows(t *testing.T) {
	casos := map[string]string{
		pcsc.EstadoSemBiblioteca: "winscard.dll",
		pcsc.EstadoSemServico:    "serviço Cartão Inteligente do Windows",
	}
	for estado, esperado := range casos {
		c := novaCenaNo(t, "windows")
		c.leitoras = pcsc.Resultado{Estado: estado, Leitoras: []pcsc.Leitora{}}
		r, texto := c.montar()
		avisos := strings.Join(r.Avisos, "\n")
		if strings.Contains(avisos+texto, "pcscd") || !strings.Contains(avisos, esperado) {
			t.Fatalf("%s: %v", estado, r.Avisos)
		}
	}
}

// No Windows, cartão lido e certificado fora da lista com o serviço de Propagação de Certificados
// parado: o aviso é o do serviço, e não o de instalar o programa do fabricante. Sem cartão, o
// serviço parado é normal (ele inicia quando o cartão entra) e não vira aviso.
func TestServicoDePropagacaoParadoNoWindows(t *testing.T) {
	parado := func() string { return ServicoParado }
	semCertificados := []assinatura.RelatorioDoProvedor{{Nome: "Windows (repositório do usuário)", Estado: assinatura.EstadoCarregado}}

	// Um certificado com a chave no próprio computador (o A1 instalado) não diz nada do cartão na
	// leitora: com ele na lista, o aviso do serviço aparece do mesmo jeito.
	c0 := novaCenaNo(t, "windows")
	chave, _ := rsa.GenerateKey(rand.Reader, 2048)
	instalado := certificadoDeTeste(t, "A1 INSTALADO:10987654321", &chave.PublicKey, false, agora.AddDate(1, 0, 0))
	c0.fontes.ServicoDePropagacao = parado
	c0.leitoras.Leitoras[0].ATR = "3B00"
	c0.provedores = []assinatura.RelatorioDoProvedor{{Nome: "Microsoft Software Key Storage Provider", Estado: assinatura.EstadoCarregado,
		Vistos: []assinatura.Certificado{{Ref: assinatura.Ref(instalado), DER: instalado, RotuloDoProvedor: assinatura.RotuloChaveNoComputador}}}}
	r0, _ := c0.montar()
	if len(r0.Certificados) != 1 || !strings.Contains(strings.Join(r0.Avisos, "\n"), "Propagação de Certificados do Windows está parado") {
		t.Fatalf("com um certificado instalado: %+v %v", r0.Certificados, r0.Avisos)
	}

	c := novaCenaNo(t, "windows")
	c.fontes.ServicoDePropagacao = parado
	c.leitoras.Leitoras[0].ATR = "3B00"
	c.provedores = semCertificados
	r, _ := c.montar()
	avisos := strings.Join(r.Avisos, "\n")
	if !strings.Contains(avisos, "Propagação de Certificados do Windows está parado") || strings.Contains(avisos, "programa do fabricante") {
		t.Fatalf("com cartão: %v", r.Avisos)
	}

	c = novaCenaNo(t, "windows")
	c.fontes.ServicoDePropagacao = parado
	c.provedores = semCertificados
	r, _ = c.montar()
	avisos = strings.Join(r.Avisos, "\n")
	if !strings.Contains(avisos, "Propagação de Certificados") || strings.Contains(avisos, "instale o SafeSign para Windows") {
		t.Fatalf("com cartão do catálogo: %v", r.Avisos)
	}

	c = novaCenaNo(t, "windows")
	c.fontes.ServicoDePropagacao = parado
	c.leitoras.Leitoras[0] = pcsc.Leitora{Nome: "Leitora USB 00 00"}
	c.provedores = semCertificados
	r, _ = c.montar()
	if strings.Contains(strings.Join(r.Avisos, "\n"), "Propagação") {
		t.Fatalf("sem cartão: %v", r.Avisos)
	}

	// Rodando, ou fora do Windows, nada se diz do serviço.
	for _, goos := range []string{"windows", "linux"} {
		c = novaCenaNo(t, goos)
		c.fontes.ServicoDePropagacao = func() string {
			if goos == "windows" {
				return ServicoRodando
			}
			return ServicoParado
		}
		c.provedores = semCertificados
		r, _ = c.montar()
		if strings.Contains(strings.Join(r.Avisos, "\n"), "Propagação") {
			t.Fatalf("%s: %v", goos, r.Avisos)
		}
	}
}

// No Windows, o ATR do catálogo dá a sugestão, mas o "não está instalado" (que olha os módulos do
// Linux) não se afirma: o provedor do fabricante no Windows ainda não foi medido.
func TestSugestaoPeloATRNoWindows(t *testing.T) {
	c := novaCenaNo(t, "windows")
	c.provedores = []assinatura.RelatorioDoProvedor{{Nome: "Windows (repositório do usuário)", Estado: assinatura.EstadoCarregado}}
	r, _ := c.montar()
	avisos := strings.Join(r.Avisos, "\n")
	if r.Leitoras[0].Sugestao != "SafeSign" || strings.Contains(avisos, "que não está instalado") || !strings.Contains(avisos, "instale o SafeSign para Windows") {
		t.Fatalf("%+v %v", r.Leitoras, r.Avisos)
	}
}

// O registro diz "Windows 10" também no Windows 11; a compilação decide.
func TestNomeDoWindows(t *testing.T) {
	casos := []struct{ produto, versao, compilacao, esperado string }{
		{"Windows 10 Pro", "22H2", "19045", "Windows 10 Pro 22H2 (compilação 19045)"},
		{"Windows 10 Pro", "23H2", "22631", "Windows 11 Pro 23H2 (compilação 22631)"},
		{"Windows 10 Enterprise", "", "22000", "Windows 11 Enterprise (compilação 22000)"},
		{"Windows 10 Home", "", "", "Windows 10 Home"},
		{"", "23H2", "22631", ""},
	}
	for _, c := range casos {
		if got := nomeDoWindows(c.produto, c.versao, c.compilacao); got != c.esperado {
			t.Errorf("%q %q %q: %q, e não %q", c.produto, c.versao, c.compilacao, got, c.esperado)
		}
	}
}

func TestCartaoDesconhecidoSemCertificado(t *testing.T) {
	c := novaCena(t)
	c.leitoras.Leitoras[0].ATR = "3B00"
	c.provedores = []assinatura.RelatorioDoProvedor{{Nome: "OpenSC", Estado: assinatura.EstadoCarregado}}
	r, _ := c.montar()
	if !strings.Contains(strings.Join(r.Avisos, "\n"), "não foi lido por nenhum programa de cartão instalado") {
		t.Fatalf("%v", r.Avisos)
	}
}

func TestOutrosAvisos(t *testing.T) {
	c := novaCena(t)
	c.leitoras = pcsc.Resultado{Estado: pcsc.EstadoOk, Leitoras: []pcsc.Leitora{{Nome: "Vazia"}}}
	c.provedores = []assinatura.RelatorioDoProvedor{{Nome: "Módulo X", Estado: assinatura.EstadoFalhou, Detalhe: "C_Initialize: CKR_GENERAL_ERROR"}}
	r, texto := c.montar()
	avisos := strings.Join(r.Avisos, "\n")
	for _, quer := range []string{"Nenhuma leitora tem cartão.", "O programa do cartão Módulo X falhou (C_Initialize: CKR_GENERAL_ERROR)."} {
		if !strings.Contains(avisos, quer) {
			t.Errorf("faltou %q em %v", quer, r.Avisos)
		}
	}
	if !strings.Contains(texto, "Leitora Vazia: sem cartão") {
		t.Errorf("texto:\n%s", texto)
	}
	c.provedores = nil
	if r, _ := c.montar(); !strings.Contains(strings.Join(r.Avisos, "\n"), "Nenhum programa de cartão (módulo PKCS#11) foi encontrado") {
		t.Errorf("sem módulo: %v", r.Avisos)
	}
	c.leitoras = pcsc.Resultado{Estado: pcsc.EstadoOk, Leitoras: []pcsc.Leitora{{Nome: "L", ComCartao: true, Mudo: true}}}
	if r, _ := c.montar(); !strings.Contains(strings.Join(r.Avisos, "\n"), "O cartão na leitora L não responde.") {
		t.Errorf("mudo: %v", r.Avisos)
	}
}

// O certificado vencido aparece com a situação e com o aviso; o de AC não aparece; o de chave EC
// aparece com a situação.
func TestSituacaoDosCertificados(t *testing.T) {
	c := novaCena(t)
	chave, _ := rsa.GenerateKey(rand.Reader, 2048)
	ec, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	vencido := certificadoDeTeste(t, "VENCIDO:11122233344", &chave.PublicKey, false, agora.Add(-time.Hour))
	ac := certificadoDeTeste(t, "AC INTERMEDIARIA", &chave.PublicKey, true, agora.AddDate(1, 0, 0))
	eliptico := certificadoDeTeste(t, "ELIPTICO:55566677788", &ec.PublicKey, false, agora.AddDate(1, 0, 0))
	for _, der := range [][]byte{vencido, ac, eliptico, []byte("não é certificado")} {
		c.provedores[0].Vistos = append(c.provedores[0].Vistos, assinatura.Certificado{Ref: assinatura.Ref(der), DER: der, RotuloDoProvedor: "SafeSign"})
	}
	r, _ := c.montar()
	situacoes := map[string]string{}
	for _, cert := range r.Certificados {
		situacoes[cert.Titular] = cert.Situacao
	}
	quer := map[string]string{
		"TITULAR DE TESTE:***********": SituacaoValido,
		"VENCIDO:***********":          SituacaoVencido,
		"ELIPTICO:***********":         SituacaoChaveNaoRSA,
		"(ilegível)":                   SituacaoIlegivel,
	}
	if len(situacoes) != len(quer) {
		t.Fatalf("certificados: %v", situacoes)
	}
	for titular, s := range quer {
		if situacoes[titular] != s {
			t.Errorf("%s: %s", titular, situacoes[titular])
		}
	}
	if !strings.Contains(strings.Join(r.Avisos, "\n"), "O certificado de VENCIDO:*********** venceu em") {
		t.Errorf("sem o aviso do vencido: %v", r.Avisos)
	}
	// A contagem do módulo é a da lista: os certificados dele, menos o da AC.
	if r.Provedores[0].Certificados != len(r.Certificados) {
		t.Errorf("o módulo conta %d, a lista tem %d", r.Provedores[0].Certificados, len(r.Certificados))
	}
}

// O pcscd que não responde não segura o diagnóstico: passado o prazo, ele diz que o pcscd não
// respondeu.
func TestPrazoDasLeitoras(t *testing.T) {
	anterior := PrazoDasLeitoras
	PrazoDasLeitoras = 100 * time.Millisecond
	defer func() { PrazoDasLeitoras = anterior }()
	trava := make(chan struct{})
	defer close(trava)
	c := novaCena(t)
	c.fontes.Leitoras = func() pcsc.Resultado { <-trava; return pcsc.Resultado{} }
	inicio := time.Now()
	r, _ := Coletar(context.Background(), c.fontes)
	if time.Since(inicio) > time.Second || r.PCSC.Estado != pcsc.EstadoFalhou || !strings.Contains(strings.Join(r.Avisos, "\n"), "o pcscd não respondeu no prazo") {
		t.Fatalf("%+v", r.PCSC)
	}
	// E o cancelamento (a entrada fechou no meio do diagnóstico) não espera o prazo.
	PrazoDasLeitoras = time.Minute
	ctx, cancelar := context.WithCancel(context.Background())
	cancelar()
	inicio = time.Now()
	if r, _ := Coletar(ctx, c.fontes); time.Since(inicio) > time.Second || r.PCSC.Detalhe != "consulta cancelada" {
		t.Fatalf("cancelado: %+v em %s", r.PCSC, time.Since(inicio))
	}
}
