package host

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Gottbrok/assinador/nativo/internal/assinatura"
	"github.com/Gottbrok/assinador/nativo/internal/bilhete"
	"github.com/Gottbrok/assinador/nativo/internal/diagnostico"
	"github.com/Gottbrok/assinador/nativo/internal/mensagens"
	"github.com/Gottbrok/assinador/nativo/internal/origem"
	"github.com/Gottbrok/assinador/nativo/internal/pcsc"
	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
)

const origemDaPagina = "https://demot.confidata.app"

var agoraFixo = time.Unix(1790000000, 0)

// provedorFalso faz o papel do cartão: assina com uma chave RSA em memória e registra o que viu.
type provedorFalso struct {
	mu        sync.Mutex
	certs     []assinatura.Certificado
	chave     *rsa.PrivateKey
	listou    int
	assinou   int
	pinVisto  string
	pinDepois []byte
	erro      error
	trocada   bool
	segurar   chan struct{}
}

func (p *provedorFalso) Listar(context.Context) ([]assinatura.Certificado, []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.listou++
	return p.certs, []string{"O programa do cartão X falhou e foi ignorado."}
}

func (p *provedorFalso) Assinar(_ context.Context, _ assinatura.Certificado, digest [32]byte, pin []byte) ([]byte, error) {
	if p.segurar != nil {
		<-p.segurar
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.assinou++
	p.pinVisto = string(pin)
	defer func() { assinatura.Zerar(pin); p.pinDepois = pin }()
	if p.erro != nil {
		return nil, p.erro
	}
	chave := p.chave
	if p.trocada {
		chave, _ = rsa.GenerateKey(rand.Reader, 2048)
	}
	return rsa.SignPKCS1v15(nil, chave, crypto.Hash(0), assinatura.DigestInfo(digest))
}

func (p *provedorFalso) Diagnosticar(context.Context) []assinatura.RelatorioDoProvedor {
	return []assinatura.RelatorioDoProvedor{{Nome: "Falso", Estado: assinatura.EstadoCarregado, Certificados: len(p.certs), Detalhe: "teste", Vistos: p.certs}}
}

type cenario struct {
	host           *Host
	provedor       *provedorFalso
	chaveDoEmissor *ecdsa.PrivateKey
	ref            string
	der            []byte
}

func certificadoRSA(t *testing.T, chave *rsa.PrivateKey, cn string, uso x509.KeyUsage, de, ate time.Time) []byte {
	t.Helper()
	modelo := &x509.Certificate{SerialNumber: big.NewInt(7), Subject: pkix.Name{CommonName: cn}, Issuer: pkix.Name{CommonName: "AC TESTE"}, NotBefore: de, NotAfter: ate, KeyUsage: uso}
	der, err := x509.CreateCertificate(rand.Reader, modelo, modelo, &chave.PublicKey, chave)
	if err != nil {
		t.Fatal(err)
	}
	return der
}

func novoCenario(t *testing.T, exigePin bool) *cenario {
	t.Helper()
	chave, _ := rsa.GenerateKey(rand.Reader, 2048)
	der := certificadoRSA(t, chave, "TITULAR DE TESTE:12345678901", x509.KeyUsageDigitalSignature, agoraFixo.Add(-time.Hour), agoraFixo.Add(time.Hour))
	ecKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ecDer := func() []byte {
		m := &x509.Certificate{SerialNumber: big.NewInt(9), Subject: pkix.Name{CommonName: "EC"}, NotBefore: agoraFixo.Add(-time.Hour), NotAfter: agoraFixo.Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature}
		d, _ := x509.CreateCertificate(rand.Reader, m, m, &ecKey.PublicKey, ecKey)
		return d
	}()
	ac := func() []byte {
		m := &x509.Certificate{SerialNumber: big.NewInt(3), Subject: pkix.Name{CommonName: "AC"}, NotBefore: agoraFixo.Add(-time.Hour), NotAfter: agoraFixo.Add(time.Hour), KeyUsage: x509.KeyUsageCertSign, BasicConstraintsValid: true, IsCA: true}
		d, _ := x509.CreateCertificate(rand.Reader, m, m, &chave.PublicKey, chave)
		return d
	}()
	p := &provedorFalso{chave: chave}
	for _, d := range [][]byte{der, ecDer, ac, []byte("não é certificado")} {
		p.certs = append(p.certs, assinatura.Certificado{Ref: assinatura.Ref(d), DER: d, Provedor: "falso", RotuloDoProvedor: "Falso", ExigePin: exigePin, EstadoDoPin: protocolo.PinOk})
	}
	emissor, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ponto, _ := emissor.PublicKey.Bytes()
	chaveDoBilhete, err := bilhete.ChaveDeJwk("confidata-teste-host", origem.Confidata, origem.AmbienteTeste,
		base64.RawURLEncoding.EncodeToString(ponto[1:33]), base64.RawURLEncoding.EncodeToString(ponto[33:]))
	if err != nil {
		t.Fatal(err)
	}
	h := &Host{
		Chamador:   origem.Chamador{Navegador: origem.Firefox, Extensao: origem.ExtensaoFirefox},
		ChamadorOk: true,
		Provedores: []assinatura.Provedor{p},
		Chaves:     []bilhete.Chave{chaveDoBilhete},
		Agora:      func() time.Time { return agoraFixo },
		Versao:     "0.1.0-teste",
		Plataforma: "linux-amd64",
	}
	return &cenario{host: h, provedor: p, chaveDoEmissor: emissor, ref: assinatura.Ref(der), der: der}
}

// emitir monta um bilhete ES256 como o servidor faria.
func (c *cenario) emitir(t *testing.T, digest, cer string, ajuste func(map[string]any)) string {
	t.Helper()
	cab, _ := json.Marshal(map[string]any{"alg": "ES256", "typ": "assinador+jws", "kid": "confidata-teste-host"})
	carga := map[string]any{
		"v": 1, "iss": "confidata", "aud": origemDaPagina, "sid": "sessao-1", "dig": digest, "cer": cer,
		"fin": "assinatura", "doc": "Contrato de teste", "org": "Organização de teste", "iat": agoraFixo.Unix() - 10, "exp": agoraFixo.Unix() + 290,
	}
	if ajuste != nil {
		ajuste(carga)
	}
	cb, _ := json.Marshal(carga)
	entrada := base64.RawURLEncoding.EncodeToString(cab) + "." + base64.RawURLEncoding.EncodeToString(cb)
	h := sha256.Sum256([]byte(entrada))
	r, s, err := ecdsa.Sign(rand.Reader, c.chaveDoEmissor, h[:])
	if err != nil {
		t.Fatal(err)
	}
	assin := make([]byte, 64)
	r.FillBytes(assin[:32])
	s.FillBytes(assin[32:])
	return entrada + "." + base64.RawURLEncoding.EncodeToString(assin)
}

func pedido(t *testing.T, op, org string, dados map[string]string) mensagens.Pedido {
	t.Helper()
	m := map[string]any{"v": 1, "id": "pedido-1", "op": op, "origem": org}
	if dados != nil {
		m["dados"] = dados
	}
	b, _ := json.Marshal(m)
	p, e := mensagens.Decodificar(b)
	if e != nil {
		t.Fatalf("pedido de teste inválido: %v", e)
	}
	return p
}

func resumo(texto string) string {
	h := sha256.Sum256([]byte(texto))
	return hex.EncodeToString(h[:])
}

func TestChamadorDesconhecidoNaoFazNada(t *testing.T) {
	c := novoCenario(t, true)
	c.host.Chamador = origem.Chamador{Navegador: origem.Firefox, Extensao: "outra@exemplo.com"}
	for _, op := range []string{"ola", "listar", "diagnostico"} {
		r := c.host.atender(context.Background(), pedido(t, op, origemDaPagina, nil))
		if r.OK || r.Erro.Codigo != protocolo.OrigemRecusada {
			t.Fatalf("%s: %+v", op, r)
		}
	}
	if c.provedor.listou != 0 {
		t.Fatal("chamador desconhecido chegou ao provedor")
	}
}

func TestOla(t *testing.T) {
	c := novoCenario(t, true)
	r := c.host.atender(context.Background(), pedido(t, "ola", "https://outro.exemplo.com", nil))
	d, ok := r.Dados.(protocolo.DadosDoOla)
	if !r.OK || !ok || d.Versao != "0.1.0-teste" || d.Protocolo != 1 || d.Plataforma != "linux-amd64" {
		t.Fatalf("%+v", r)
	}
}

func TestListarFiltraEAvisa(t *testing.T) {
	c := novoCenario(t, true)
	r := c.host.atender(context.Background(), pedido(t, "listar", origemDaPagina, nil))
	d := r.Dados.(protocolo.DadosDoListar)
	if len(d.Certificados) != 1 || d.Certificados[0].Ref != c.ref || !d.Certificados[0].ExigePin {
		t.Fatalf("lista: %+v", d.Certificados)
	}
	der, err := base64.StdEncoding.DecodeString(d.Certificados[0].DER)
	if err != nil || !bytes.Equal(der, c.der) {
		t.Fatal("DER em base64 padrão diferente do certificado")
	}
	if strings.Join(d.Avisos, "|") != "O programa do cartão X falhou e foi ignorado.|"+AvisoAlgoritmo+"|"+AvisoIlegivel {
		t.Fatalf("avisos: %v", d.Avisos)
	}
	// Página fora dos padrões não lista, e nem chega ao cartão.
	c.provedor.listou = 0
	r = c.host.atender(context.Background(), pedido(t, "listar", "https://evil.example.com", nil))
	if r.OK || r.Erro.Codigo != protocolo.OrigemRecusada || c.provedor.listou != 0 {
		t.Fatalf("origem estranha: %+v", r)
	}
}

// O bilhete é conferido ANTES de o provedor ser consultado: pedido forjado não carrega biblioteca
// de fabricante.
func TestBilheteAntesDoProvedor(t *testing.T) {
	c := novoCenario(t, true)
	digest := resumo("documento")
	casos := map[string]struct {
		bilhete string
		codigo  protocolo.Codigo
	}{
		"sem assinatura válida": {c.emitir(t, digest, c.ref, nil)[:40] + "x.y.z", protocolo.BilheteInvalido},
		"outro resumo":          {c.emitir(t, resumo("outro"), c.ref, nil), protocolo.DigestDivergente},
		"outro certificado":     {c.emitir(t, digest, resumo("outro"), nil), protocolo.CertificadoDivergente},
		"outra origem":          {c.emitir(t, digest, c.ref, func(m map[string]any) { m["aud"] = "https://outra.confidata.app" }), protocolo.OrigemRecusada},
		"relogio":               {c.emitir(t, digest, c.ref, func(m map[string]any) { m["iat"] = agoraFixo.Unix() + 2000; m["exp"] = agoraFixo.Unix() + 2300 }), protocolo.Relogio},
	}
	for nome, caso := range casos {
		for _, op := range []string{"conferir", "assinar"} {
			dados := map[string]string{"ref": c.ref, "digest": digest, "bilhete": caso.bilhete}
			if op == "assinar" {
				dados["pin"] = "1234"
			}
			r := c.host.atender(context.Background(), pedido(t, op, origemDaPagina, dados))
			if r.OK || r.Erro.Codigo != caso.codigo {
				t.Errorf("%s/%s: %+v", nome, op, r.Erro)
			}
		}
	}
	if c.provedor.listou != 0 || c.provedor.assinou != 0 {
		t.Fatalf("bilhete recusado chegou ao provedor: listou %d, assinou %d", c.provedor.listou, c.provedor.assinou)
	}
}

func TestConferirMostraOBilheteEOCertificadoSemCpf(t *testing.T) {
	c := novoCenario(t, true)
	digest := resumo("documento")
	r := c.host.atender(context.Background(), pedido(t, "conferir", origemDaPagina, map[string]string{"ref": c.ref, "digest": digest, "bilhete": c.emitir(t, digest, c.ref, nil)}))
	d, ok := r.Dados.(protocolo.DadosDoConferir)
	if !r.OK || !ok {
		t.Fatalf("%+v", r)
	}
	if d.Emissor != "confidata" || d.Organizacao != "Organização de teste" || d.Documento != "Contrato de teste" || d.Finalidade != "assinatura" {
		t.Fatalf("bilhete: %+v", d)
	}
	if d.Certificado.Assunto != "TITULAR DE TESTE:***********" || d.Certificado.Emissor != "TITULAR DE TESTE:12345678901" && d.Certificado.Emissor == "" {
		t.Fatalf("certificado: %+v", d.Certificado)
	}
	if d.ExpiraEm != time.Unix(agoraFixo.Unix()+290, 0).UTC().Format(time.RFC3339) {
		t.Fatalf("expira: %s", d.ExpiraEm)
	}
}

func TestAssinarLevaOPinSoQuandoPrecisaEZera(t *testing.T) {
	c := novoCenario(t, true)
	digest := resumo("documento")
	jws := c.emitir(t, digest, c.ref, nil)

	// Sem PIN, ou PIN vazio, num token que exige: nada vai ao cartão.
	for _, dados := range []map[string]string{
		{"ref": c.ref, "digest": digest, "bilhete": jws},
		{"ref": c.ref, "digest": digest, "bilhete": jws, "pin": ""},
	} {
		r := c.host.atender(context.Background(), pedido(t, "assinar", origemDaPagina, dados))
		if r.OK || r.Erro.Codigo != protocolo.Protocolo || c.provedor.assinou != 0 {
			t.Fatalf("sem PIN: %+v, assinou %d", r, c.provedor.assinou)
		}
	}

	p := pedido(t, "assinar", origemDaPagina, map[string]string{"ref": c.ref, "digest": digest, "bilhete": jws, "pin": "4321"})
	pinDoPedido := p.Pin[:cap(p.Pin)]
	r := c.host.atender(context.Background(), p)
	if !r.OK {
		t.Fatalf("%+v", r.Erro)
	}
	if c.provedor.pinVisto != "4321" {
		t.Fatalf("o provedor viu %q", c.provedor.pinVisto)
	}
	if !bytes.Equal(pinDoPedido, make([]byte, len(pinDoPedido))) {
		t.Fatal("o PIN do pedido ficou no buffer depois da operação")
	}
	assin, _ := base64.StdEncoding.DecodeString(r.Dados.(protocolo.DadosDoAssinar).Assinatura)
	var d [32]byte
	hex.Decode(d[:], []byte(digest))
	if err := rsa.VerifyPKCS1v15(&c.provedor.chave.PublicKey, crypto.SHA256, d[:], assin); err != nil {
		t.Fatalf("a assinatura devolvida não confere: %v", err)
	}

	// Token com caminho protegido (o leitor pede o PIN): o PIN que vier não vai ao provedor.
	c2 := novoCenario(t, false)
	jws2 := c2.emitir(t, digest, c2.ref, nil)
	r = c2.host.atender(context.Background(), pedido(t, "assinar", origemDaPagina, map[string]string{"ref": c2.ref, "digest": digest, "bilhete": jws2, "pin": "9999"}))
	if !r.OK || c2.provedor.pinVisto != "" {
		t.Fatalf("caminho protegido: %+v, viu %q", r, c2.provedor.pinVisto)
	}
}

func TestAssinarConfereOQueODispositivoDevolveu(t *testing.T) {
	c := novoCenario(t, true)
	c.provedor.trocada = true
	digest := resumo("documento")
	r := c.host.atender(context.Background(), pedido(t, "assinar", origemDaPagina, map[string]string{"ref": c.ref, "digest": digest, "bilhete": c.emitir(t, digest, c.ref, nil), "pin": "1"}))
	if r.OK || r.Erro.Codigo != protocolo.Interno {
		t.Fatalf("assinatura de outra chave passou: %+v", r)
	}
}

func TestAssinarRepassaOErroDoDispositivo(t *testing.T) {
	c := novoCenario(t, true)
	c.provedor.erro = protocolo.PinErrado(protocolo.TentativasUltima)
	digest := resumo("documento")
	r := c.host.atender(context.Background(), pedido(t, "assinar", origemDaPagina, map[string]string{"ref": c.ref, "digest": digest, "bilhete": c.emitir(t, digest, c.ref, nil), "pin": "1"}))
	b, _ := json.Marshal(r)
	if !strings.Contains(string(b), `"erro":{"codigo":"pin-incorreto","detalhe":{"tentativas":"ultima"}}`) {
		t.Fatalf("%s", b)
	}
	c.provedor.erro = errors.New("qualquer")
	r = c.host.atender(context.Background(), pedido(t, "assinar", origemDaPagina, map[string]string{"ref": c.ref, "digest": digest, "bilhete": c.emitir(t, digest, c.ref, nil), "pin": "1"}))
	if r.OK || r.Erro.Codigo != protocolo.Interno {
		t.Fatalf("%+v", r)
	}
}

func TestCertificadoVencidoOuAusenteNaoAssina(t *testing.T) {
	c := novoCenario(t, true)
	digest := resumo("documento")
	c.host.Agora = func() time.Time { return agoraFixo.Add(2 * time.Hour) }
	jws := c.emitir(t, digest, c.ref, func(m map[string]any) { m["iat"] = agoraFixo.Unix() + 7000; m["exp"] = agoraFixo.Unix() + 7300 })
	r := c.host.atender(context.Background(), pedido(t, "assinar", origemDaPagina, map[string]string{"ref": c.ref, "digest": digest, "bilhete": jws, "pin": "1"}))
	if r.OK || r.Erro.Codigo != protocolo.CertificadoNaoEncontrado || c.provedor.assinou != 0 {
		t.Fatalf("vencido: %+v", r)
	}
	c.host.Agora = func() time.Time { return agoraFixo }
	outra := resumo("não está no cartão")
	r = c.host.atender(context.Background(), pedido(t, "conferir", origemDaPagina, map[string]string{"ref": outra, "digest": digest, "bilhete": c.emitir(t, digest, outra, nil)}))
	if r.OK || r.Erro.Codigo != protocolo.CertificadoNaoEncontrado {
		t.Fatalf("ausente: %+v", r)
	}
}

// A ref que o provedor declara não é aceita: o host a recalcula do DER. Um provedor que pusesse a
// ref de outro certificado não faria o bilhete daquele valer para este.
func TestRefRecalculadaDoDer(t *testing.T) {
	c := novoCenario(t, true)
	outroDer := c.provedor.certs[1].DER
	c.provedor.certs[0].DER = outroDer // a ref continua a do titular, o DER é outro
	digest := resumo("documento")
	r := c.host.atender(context.Background(), pedido(t, "conferir", origemDaPagina, map[string]string{"ref": c.ref, "digest": digest, "bilhete": c.emitir(t, digest, c.ref, nil)}))
	if r.OK || r.Erro.Codigo != protocolo.CertificadoNaoEncontrado {
		t.Fatalf("ref que não é do DER passou: %+v", r)
	}
	lista := c.host.atender(context.Background(), pedido(t, "listar", origemDaPagina, nil)).Dados.(protocolo.DadosDoListar)
	for _, cert := range lista.Certificados {
		if cert.Ref == c.ref {
			t.Fatal("a lista trouxe a ref que não é do DER")
		}
	}
}

// A operação `diagnostico` responde com o relatório do pacote `diagnostico`, que lê as leitoras
// pela função que o host recebeu.
func TestDiagnostico(t *testing.T) {
	c := novoCenario(t, true)
	c.host.Leitoras = func() pcsc.Resultado {
		return pcsc.Resultado{Estado: pcsc.EstadoSemServico, Detalhe: "0x8010001D", Leitoras: []pcsc.Leitora{}}
	}
	r := c.host.atender(context.Background(), pedido(t, "diagnostico", origemDaPagina, nil))
	d := r.Dados.(protocolo.DadosDoDiagnostico)
	// O diagnóstico conta os certificados do módulo menos o da AC: dos 4 do provedor falso, 3.
	if !r.OK || !strings.Contains(d.Texto, "Assinador 0.1.0-teste") || !strings.Contains(d.Texto, "Programa do cartão Falso: carregado, 3 certificado(s)") || !strings.Contains(d.Texto, "O serviço pcscd não está rodando") {
		t.Fatalf("%+v", d)
	}
	if _, ok := d.Relatorio.(diagnostico.Relatorio); !ok {
		t.Fatalf("relatório de outro tipo: %T", d.Relatorio)
	}
}

// canal liga o teste ao host pelo quadro de native messaging.
type canal struct {
	escreve *io.PipeWriter
	le      *io.PipeReader
}

func iniciar(t *testing.T, h *Host) (*canal, chan error) {
	t.Helper()
	leEntrada, escreveEntrada := io.Pipe()
	leSaida, escreveSaida := io.Pipe()
	h.Entrada = leEntrada
	h.Saida = escreveSaida
	fim := make(chan error, 1)
	go func() {
		fim <- h.Executar(context.Background())
		escreveSaida.Close()
	}()
	return &canal{escreve: escreveEntrada, le: leSaida}, fim
}

func (c *canal) mandar(t *testing.T, corpo string) {
	t.Helper()
	if err := mensagens.EscreverQuadro(c.escreve, []byte(corpo), 1<<30); err != nil {
		t.Fatal(err)
	}
}

func (c *canal) receber(t *testing.T) map[string]any {
	t.Helper()
	b, err := mensagens.LerQuadro(c.le, protocolo.SaidaMaxima)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func codigoDe(m map[string]any) string {
	if e, ok := m["erro"].(map[string]any); ok {
		return e["codigo"].(string)
	}
	return ""
}

func TestExecutarUmaOperacaoPorVez(t *testing.T) {
	c := novoCenario(t, true)
	c.provedor.segurar = make(chan struct{})
	can, fim := iniciar(t, c.host)
	digest := resumo("documento")
	assinar, _ := json.Marshal(map[string]any{"v": 1, "id": "primeiro", "op": "assinar", "origem": origemDaPagina,
		"dados": map[string]string{"ref": c.ref, "digest": digest, "bilhete": c.emitir(t, digest, c.ref, nil), "pin": "1"}})
	// O laço trata os quadros na ordem em que chegam: o primeiro já marcou `ocupado` quando o
	// segundo é lido.
	can.mandar(t, string(assinar))
	can.mandar(t, `{"v":1,"id":"segundo","op":"ola","origem":"https://ushield.app"}`)
	r := can.receber(t)
	if r["id"] != "segundo" || codigoDe(r) != "ocupado" {
		t.Fatalf("concorrente: %v", r)
	}
	close(c.provedor.segurar)
	if r = can.receber(t); r["id"] != "primeiro" || r["ok"] != true {
		t.Fatalf("primeiro: %v", r)
	}
	can.mandar(t, `{"v":1,"id":"terceiro","op":"ola","origem":"https://ushield.app"}`)
	if r = can.receber(t); r["id"] != "terceiro" || r["ok"] != true {
		t.Fatalf("depois de livre: %v", r)
	}
	// JSON fora do protocolo recebe `protocolo`, e o host segue.
	can.mandar(t, `{"v":1,"id":"quarto","op":"ola","origem":"https://ushield.app","x":1}`)
	if r = can.receber(t); r["id"] != "quarto" || codigoDe(r) != "protocolo" {
		t.Fatalf("fora do protocolo: %v", r)
	}
	can.mandar(t, `lixo`)
	if r = can.receber(t); r["id"] != "" || codigoDe(r) != "protocolo" {
		t.Fatalf("lixo: %v", r)
	}
	can.escreve.Close()
	if err := <-fim; err != nil {
		t.Fatalf("a entrada fechada devia encerrar sem erro: %v", err)
	}
}

// O cancelamento (que é como o SIGTERM chega ao host) encerra o programa sem esperar a entrada
// fechar; antes, o sinal era engolido e o processo seguia vivo.
func TestExecutarEncerraNoCancelamento(t *testing.T) {
	c := novoCenario(t, true)
	leEntrada, escreveEntrada := io.Pipe()
	defer escreveEntrada.Close()
	c.host.Entrada = leEntrada
	c.host.Saida = io.Discard
	ctx, cancelar := context.WithCancel(context.Background())
	fim := make(chan error, 1)
	go func() { fim <- c.host.Executar(ctx) }()
	cancelar()
	select {
	case err := <-fim:
		if err != nil {
			t.Fatalf("o cancelamento devia encerrar sem erro: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("o host não encerrou no cancelamento")
	}
}

func TestExecutarEncerraComMensagemGrande(t *testing.T) {
	c := novoCenario(t, true)
	can, fim := iniciar(t, c.host)
	go func() { _ = mensagens.EscreverQuadro(can.escreve, make([]byte, protocolo.EntradaMaxima+1), 1<<30) }()
	r := can.receber(t)
	if codigoDe(r) != "protocolo" {
		t.Fatalf("%v", r)
	}
	if err := <-fim; !errors.Is(err, mensagens.ErrQuadroGrande) {
		t.Fatalf("%v", err)
	}
}
