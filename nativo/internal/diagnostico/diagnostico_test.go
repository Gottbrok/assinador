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

func novaCena(t *testing.T) *cena {
	t.Helper()
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
	return Montar(c.fontes, c.leitoras, c.provedores)
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
	if !strings.Contains(strings.Join(r.Avisos, "\n"), "O serviço pcscd não está rodando") || !strings.Contains(texto, "sudo systemctl start pcscd") {
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
}
