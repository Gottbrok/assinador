// Package diagnostico monta o que o suporte recebe: o sistema, o programa, as leitoras e o ATR de
// cada cartão (com a sugestão do programa do fabricante, quando o ATR está no catálogo medido), os
// módulos PKCS#11 com o estado de cada um, os certificados (o nome mascarado) e os avisos, em frase
// (§3.5 do plano). Sem CPF: o CN ICP-Brasil é `NOME:CPF`, e os dígitos saem trocados por `*`.
//
// O mesmo relatório responde à operação `diagnostico` da extensão e ao modo `assinador
// diagnostico` do terminal.
package diagnostico

import (
	"context"
	"crypto/x509"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Gottbrok/assinador/nativo/internal/assinatura"
	"github.com/Gottbrok/assinador/nativo/internal/catalogo"
	"github.com/Gottbrok/assinador/nativo/internal/pcsc"
	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
)

// Relatorio é o `relatorio` do diagnóstico.
type Relatorio struct {
	Programa     Programa                         `json:"programa"`
	Sistema      string                           `json:"sistema"`
	PCSC         EstadoDoPCSC                     `json:"pcsc"`
	Leitoras     []Leitora                        `json:"leitoras"`
	Provedores   []assinatura.RelatorioDoProvedor `json:"provedores"`
	Certificados []Certificado                    `json:"certificados"`
	Avisos       []string                         `json:"avisos"`
}

// Programa é a versão e a plataforma deste programa.
type Programa struct {
	Versao     string `json:"versao"`
	Protocolo  int    `json:"protocolo"`
	Plataforma string `json:"plataforma"`
}

// EstadoDoPCSC é o resultado da consulta às leitoras.
type EstadoDoPCSC struct {
	Estado  string `json:"estado"`
	Detalhe string `json:"detalhe,omitempty"`
}

// Leitora é uma leitora, o cartão nela e, pelo ATR, o programa que o lê.
type Leitora struct {
	Nome      string `json:"nome"`
	ComCartao bool   `json:"comCartao"`
	Mudo      bool   `json:"mudo,omitempty"`
	ATR       string `json:"atr,omitempty"`
	Cartao    string `json:"cartao,omitempty"`
	Sugestao  string `json:"sugestao,omitempty"`
}

// Situações de um certificado no relatório.
const (
	SituacaoValido         = "valido"
	SituacaoVencido        = "vencido"
	SituacaoAindaNaoValido = "ainda-nao-valido"
	SituacaoChaveNaoRSA    = "chave-nao-rsa"
	SituacaoSemUso         = "sem-uso-de-assinatura"
	SituacaoIlegivel       = "ilegivel"
)

// Certificado é um certificado visto num módulo, resumido: o titular com os dígitos mascarados.
type Certificado struct {
	Titular   string `json:"titular"`
	Emissor   string `json:"emissor,omitempty"`
	ValidoAte string `json:"validoAte,omitempty"`
	Situacao  string `json:"situacao"`
	Provedor  string `json:"provedor"`
	Leitor    string `json:"leitor,omitempty"`
}

// Fontes são de onde o diagnóstico lê. `Leitoras` é injetável para o teste; o programa passa
// `pcsc.Consultar`.
type Fontes struct {
	Provedores []assinatura.Provedor
	Leitoras   func() pcsc.Resultado
	// ServicoDePropagacao diz o estado do serviço de Propagação de Certificados (`CertPropSvc`) do
	// Windows, que põe o certificado do cartão no repositório do usuário (`Servico*`); nulo ou vazio
	// fora dele.
	ServicoDePropagacao func() string
	Agora               func() time.Time
	Versao              string
	Plataforma          string
	Sistema             string
	ATRs                []catalogo.ATR
	Modulos             []catalogo.Modulo
}

// Estados do serviço de Propagação de Certificados do Windows. `desconhecido` é o serviço que não
// pôde ser consultado: não vira aviso.
const (
	ServicoRodando      = "rodando"
	ServicoParado       = "parado"
	ServicoDesconhecido = "desconhecido"
)

// PrazoDasLeitoras é quanto o diagnóstico espera o `pcscd`. Passado o prazo, o relatório diz que
// ele não respondeu, e a consulta que ficou presa é abandonada.
var PrazoDasLeitoras = 5 * time.Second

// Coletar consulta as leitoras e os provedores e monta o relatório e o texto.
func Coletar(ctx context.Context, f Fontes) (Relatorio, string) {
	// As leitoras são consultadas AO MESMO TEMPO que os módulos: em série, o prazo dos filhos e o do
	// pcscd se somavam perto do prazo da página.
	leituras := make(chan pcsc.Resultado, 1)
	go func() { leituras <- consultarComPrazo(ctx, f.Leitoras) }()
	var provedores []assinatura.RelatorioDoProvedor
	for _, p := range f.Provedores {
		provedores = append(provedores, p.Diagnosticar(ctx)...)
	}
	return Montar(f, <-leituras, provedores)
}

// consultarComPrazo espera o pcscd até o prazo ou até o cancelamento. Passado o prazo, a consulta
// que ficou presa no C é abandonada: não há como interrompê-la, e o processo segue.
func consultarComPrazo(ctx context.Context, consultar func() pcsc.Resultado) pcsc.Resultado {
	if consultar == nil {
		return pcsc.Resultado{Estado: pcsc.EstadoSemBiblioteca, Leitoras: []pcsc.Leitora{}}
	}
	pronto := make(chan pcsc.Resultado, 1)
	go func() { pronto <- consultar() }()
	select {
	case r := <-pronto:
		return r
	case <-time.After(PrazoDasLeitoras):
		servico := "o pcscd"
		if sistemaOperacional == "windows" {
			servico = "o serviço Cartão Inteligente"
		}
		return pcsc.Resultado{Estado: pcsc.EstadoFalhou, Detalhe: servico + " não respondeu no prazo", Leitoras: []pcsc.Leitora{}}
	case <-ctx.Done():
		return pcsc.Resultado{Estado: pcsc.EstadoFalhou, Detalhe: "consulta cancelada", Leitoras: []pcsc.Leitora{}}
	}
}

// sistemaOperacional decide a frase da biblioteca do PC/SC ausente (variável para o teste).
var sistemaOperacional = runtime.GOOS

// limite de pontos de código de um texto que veio do aparelho ou do certificado.
const tamanhoMaximoDoTexto = 256

// limpar tira de um texto que veio de FORA (o nome da leitora, o CN do certificado, o que o módulo
// declara) os caracteres de controle e os que reordenam o texto, e o corta: o texto do diagnóstico
// vai ao terminal do suporte e à página, e um aparelho malicioso não pode forjar linha nem mandar
// sequência de escape.
func limpar(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		if unicode.IsControl(r) || unicode.In(r, unicode.Bidi_Control) || r == 0x2028 || r == 0x2029 || r == 0xfeff {
			continue
		}
		if n == tamanhoMaximoDoTexto {
			break
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}

// mascararDocumentos troca por `*` toda sequência de 11 ou mais dígitos (o CPF tem 11 e o CNPJ 14):
// é o que vale para o emissor, que numa AC não tem documento, mas num certificado forjado pode ter.
func mascararDocumentos(s string) string {
	runas := []rune(s)
	for i := 0; i < len(runas); {
		if runas[i] < '0' || runas[i] > '9' {
			i++
			continue
		}
		j := i
		for j < len(runas) && runas[j] >= '0' && runas[j] <= '9' {
			j++
		}
		if j-i >= 11 {
			for k := i; k < j; k++ {
				runas[k] = '*'
			}
		}
		i = j
	}
	return string(runas)
}

// doCatalogo diz se o relatório de um módulo é o do módulo `m` do catálogo: pelo rótulo (o módulo
// achado no caminho medido, ou reconhecido pelo nome do arquivo) ou pelo fabricante que ele
// declarou no C_GetInfo, que é medido.
func doCatalogo(p assinatura.RelatorioDoProvedor, m catalogo.Modulo) bool {
	return p.Nome == m.Rotulo || (m.Fabricante != "" && p.Fabricante == m.Fabricante)
}

// Montar monta o relatório e o texto a partir do que foi lido. É pura: o teste a exercita com
// leituras forjadas.
func Montar(f Fontes, leitoras pcsc.Resultado, provedores []assinatura.RelatorioDoProvedor) (Relatorio, string) {
	agora := time.Now()
	if f.Agora != nil {
		agora = f.Agora()
	}
	r := Relatorio{
		Programa:     Programa{Versao: f.Versao, Protocolo: protocolo.Versao, Plataforma: f.Plataforma},
		Sistema:      f.Sistema,
		PCSC:         EstadoDoPCSC{Estado: leitoras.Estado, Detalhe: leitoras.Detalhe},
		Leitoras:     []Leitora{},
		Provedores:   []assinatura.RelatorioDoProvedor{},
		Certificados: []Certificado{},
		Avisos:       []string{},
	}
	// Cada módulo, com o que veio de fora limpo, e com a contagem dos SEUS certificados (menos os de
	// AC, que são a cadeia guardada no cartão): é essa contagem que decide o "instalado, mas não
	// achou certificado", e não a lista fundida, em que o certificado fica com o primeiro módulo
	// que o viu.
	vistos := map[string]bool{}
	for _, p := range provedores {
		item := p
		item.Nome, item.Caminho, item.Fabricante, item.Detalhe = limpar(p.Nome), limpar(p.Caminho), limpar(p.Fabricante), limpar(p.Detalhe)
		if p.Estado == assinatura.EstadoCarregado {
			item.Certificados = 0
			for _, c := range p.Vistos {
				resumo, ok := resumir(c, agora)
				if !ok {
					continue
				}
				item.Certificados++
				if !vistos[c.Ref] {
					vistos[c.Ref] = true
					r.Certificados = append(r.Certificados, resumo)
				}
			}
		}
		item.Vistos = nil
		r.Provedores = append(r.Provedores, item)
	}
	// modulo diz se o módulo `m` do catálogo carregou e quantos certificados ele mesmo leu.
	modulo := func(m catalogo.Modulo) (carregou bool, certificados int) {
		for _, p := range r.Provedores {
			if p.Estado == assinatura.EstadoCarregado && doCatalogo(p, m) {
				carregou = true
				certificados += p.Certificados
			}
		}
		return carregou, certificados
	}

	avisar := func(formato string, args ...any) { r.Avisos = append(r.Avisos, fmt.Sprintf(formato, args...)) }
	windows := sistemaOperacional == "windows"
	switch leitoras.Estado {
	case pcsc.EstadoSemBiblioteca:
		switch sistemaOperacional {
		case "linux":
			avisar("A biblioteca do PC/SC não está instalada, e sem ela nenhuma leitora aparece. Instale o pacote pcscd (no Fedora, pcsc-lite).")
		case "windows":
			avisar("O componente de cartão inteligente do Windows (winscard.dll) não abriu, e sem ele nenhuma leitora aparece.")
		default:
			avisar("O Assinador ainda não lê as leitoras de cartão neste sistema.")
		}
	case pcsc.EstadoSemServico:
		if windows {
			// O Windows mantém o serviço Cartão Inteligente rodando só enquanto há leitora conectada:
			// "sem serviço" ali é, quase sempre, "sem leitora".
			avisar("Nenhuma leitora de cartão foi encontrada, ou o serviço Cartão Inteligente do Windows está parado. Confira o cabo USB da leitora ou do token.")
		} else {
			avisar("O serviço pcscd não está rodando, e sem ele nenhuma leitora aparece. Para iniciar: sudo systemctl start pcscd.")
		}
	case pcsc.EstadoSemLeitora:
		avisar("Nenhuma leitora de cartão foi encontrada. Confira o cabo USB da leitora ou do token.")
	case pcsc.EstadoFalhou:
		avisar("O PC/SC não respondeu como devia (%s).", limpar(leitoras.Detalhe))
	}
	r.PCSC.Detalhe = limpar(r.PCSC.Detalhe)
	// No Windows, o certificado do cartão chega ao repositório do usuário pelo serviço de Propagação
	// de Certificados, que inicia por gatilho quando um cartão entra (parado sem cartão é normal).
	// Cartão lido e certificado fora da lista, com ele parado, é o caso mais provável de "não
	// aparece" (§3.5 do plano): o aviso dele substitui o de instalar o programa do fabricante.
	propagacaoParada := windows && f.ServicoDePropagacao != nil && f.ServicoDePropagacao() == ServicoParado
	algumCartao := false
	for _, l := range leitoras.Leitoras {
		nome := limpar(l.Nome)
		item := Leitora{Nome: nome, ComCartao: l.ComCartao, Mudo: l.Mudo, ATR: limpar(l.ATR)}
		if l.ComCartao {
			algumCartao = true
			if m, a, ok := catalogo.ModuloDoATR(l.ATR, f.ATRs, f.Modulos); ok {
				item.Cartao, item.Sugestao = a.Cartao, m.Rotulo
				carregou, certificados := modulo(m)
				switch {
				case windows:
					// O catálogo de módulos é o do Linux. No Windows quem lê o cartão é o provedor do
					// fabricante, cujo nome ainda não foi medido (regra 4): a sugestão vale, e o "não
					// está instalado" não se afirma.
					if len(r.Certificados) == 0 && !propagacaoParada {
						avisar("O cartão na leitora %s (%s) usa o %s. Se o certificado não aparece na lista, instale o %s para Windows.", nome, a.Cartao, m.Rotulo, m.Rotulo)
					}
				case !carregou:
					avisar("O cartão na leitora %s (%s) usa o %s, que não está instalado. Instale o %s.", nome, a.Cartao, m.Rotulo, m.Rotulo)
				case certificados == 0:
					avisar("O %s está instalado, mas não achou certificado no cartão da leitora %s.", m.Rotulo, nome)
				}
			} else if len(r.Certificados) == 0 && !propagacaoParada {
				avisar("O cartão na leitora %s não foi lido por nenhum programa de cartão instalado. Ele precisa do programa do fabricante (por exemplo, o SafeSign ou o SafeNet).", nome)
			}
			if l.Mudo {
				avisar("O cartão na leitora %s não responde. Tire o cartão e coloque de novo.", nome)
			}
		}
		r.Leitoras = append(r.Leitoras, item)
	}
	if leitoras.Estado == pcsc.EstadoOk && !algumCartao && len(r.Certificados) == 0 {
		avisar("Nenhuma leitora tem cartão. Coloque o cartão na leitora (ou conecte o token).")
	}
	if propagacaoParada && algumCartao && len(r.Certificados) == 0 {
		avisar("O serviço Propagação de Certificados do Windows está parado, e com ele parado o certificado do cartão não entra na lista. Para iniciar: abra Serviços (services.msc), Propagação de Certificados, Iniciar.")
	}
	algumModulo := false
	for _, p := range r.Provedores {
		switch p.Estado {
		case assinatura.EstadoCarregado:
			algumModulo = true
		case assinatura.EstadoFalhou:
			algumModulo = true
			avisar("O programa do cartão %s falhou (%s).", p.Nome, p.Detalhe)
		}
	}
	if !algumModulo {
		avisar("Nenhum programa de cartão (módulo PKCS#11) foi encontrado neste computador. Instale o do fabricante do cartão ou do token.")
	}
	for _, c := range r.Certificados {
		if c.Situacao == SituacaoVencido {
			avisar("O certificado de %s venceu em %s: ele aparece na lista, mas não assina.", c.Titular, c.ValidoAte)
		}
	}
	return r, texto(r)
}

// resumir tira do DER o que o suporte precisa, com o titular mascarado. Certificado de AC não
// entra (é a cadeia guardada no cartão, não o da pessoa).
func resumir(c assinatura.Certificado, agora time.Time) (Certificado, bool) {
	saida := Certificado{Provedor: limpar(c.RotuloDoProvedor), Leitor: limpar(c.Leitor)}
	x, err := x509.ParseCertificate(c.DER)
	if err != nil {
		saida.Titular, saida.Situacao = "(ilegível)", SituacaoIlegivel
		return saida, true
	}
	if x.IsCA {
		return Certificado{}, false
	}
	saida.Titular = limpar(assinatura.Mascarar(x.Subject.CommonName))
	saida.Emissor = limpar(mascararDocumentos(x.Issuer.CommonName))
	if x.Issuer.CommonName == x.Subject.CommonName {
		// Autoassinado: o emissor é o próprio titular, e leva o documento dele.
		saida.Emissor = saida.Titular
	}
	saida.ValidoAte = x.NotAfter.UTC().Format(time.DateOnly)
	ok, motivo := assinatura.Listavel(x)
	switch {
	case !ok && motivo == assinatura.ForaDaListaAlgoritmo:
		saida.Situacao = SituacaoChaveNaoRSA
	case !ok:
		saida.Situacao = SituacaoSemUso
	case agora.After(x.NotAfter):
		saida.Situacao = SituacaoVencido
	case agora.Before(x.NotBefore):
		saida.Situacao = SituacaoAindaNaoValido
	default:
		saida.Situacao = SituacaoValido
	}
	return saida, true
}

var nomesDaSituacao = map[string]string{
	SituacaoValido:         "válido",
	SituacaoVencido:        "vencido",
	SituacaoAindaNaoValido: "ainda não vale",
	SituacaoChaveNaoRSA:    "chave que não é RSA",
	SituacaoSemUso:         "sem uso de assinatura",
	SituacaoIlegivel:       "ilegível",
}

var nomesDoEstadoDoPCSC = map[string]string{
	pcsc.EstadoOk:            "ok",
	pcsc.EstadoSemBiblioteca: "biblioteca não instalada",
	pcsc.EstadoSemServico:    "serviço parado",
	pcsc.EstadoSemLeitora:    "nenhuma leitora",
	pcsc.EstadoFalhou:        "falhou",
}

func texto(r Relatorio) string {
	var b strings.Builder
	linha := func(formato string, args ...any) { fmt.Fprintf(&b, formato+"\n", args...) }
	linha("Assinador %s (%s), protocolo %d", r.Programa.Versao, r.Programa.Plataforma, r.Programa.Protocolo)
	if r.Sistema != "" {
		linha("Sistema: %s", r.Sistema)
	}
	pcscTexto := nomesDoEstadoDoPCSC[r.PCSC.Estado]
	if r.PCSC.Detalhe != "" {
		pcscTexto += " (" + r.PCSC.Detalhe + ")"
	}
	linha("PC/SC: %s", pcscTexto)
	for _, l := range r.Leitoras {
		switch {
		case !l.ComCartao:
			linha("Leitora %s: sem cartão", l.Nome)
		case l.Sugestao != "":
			linha("Leitora %s: com cartão, ATR %s, %s (usa o %s)", l.Nome, l.ATR, l.Cartao, l.Sugestao)
		default:
			linha("Leitora %s: com cartão, ATR %s", l.Nome, l.ATR)
		}
	}
	for _, p := range r.Provedores {
		origem := ""
		if p.Origem != "" {
			origem = " [" + p.Origem + "]"
		}
		switch p.Estado {
		case assinatura.EstadoCarregado:
			linha("Programa do cartão %s: carregado, %d certificado(s) (%s)%s", p.Nome, p.Certificados, p.Detalhe, origem)
		case assinatura.EstadoFalhou:
			linha("Programa do cartão %s: falhou (%s)%s", p.Nome, p.Detalhe, origem)
		default:
			linha("Programa do cartão %s: não instalado (%s)%s", p.Nome, p.Caminho, origem)
		}
	}
	for _, c := range r.Certificados {
		detalhe := []string{c.Titular}
		if c.Emissor != "" {
			detalhe = append(detalhe, "emissor "+c.Emissor)
		}
		if c.ValidoAte != "" {
			detalhe = append(detalhe, "válido até "+c.ValidoAte)
		}
		detalhe = append(detalhe, nomesDaSituacao[c.Situacao], c.Provedor)
		if c.Leitor != "" {
			detalhe = append(detalhe, "leitora "+c.Leitor)
		}
		linha("Certificado: %s", strings.Join(detalhe, " · "))
	}
	if len(r.Avisos) > 0 {
		linha("Avisos:")
		for _, a := range r.Avisos {
			linha("- %s", a)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// primeiraCompilacaoDoWindows11 é o número de compilação em que o Windows 11 começa. O registro
// continua dizendo "Windows 10" no `ProductName` do Windows 11, e é a compilação que os distingue.
const primeiraCompilacaoDoWindows11 = 22000

// nomeDoWindows monta o nome do sistema a partir do registro (`ProductName`, `DisplayVersion` e
// `CurrentBuild`), trocando o "Windows 10" pelo "Windows 11" quando a compilação é do 11.
func nomeDoWindows(produto, versao, compilacao string) string {
	if produto == "" {
		return ""
	}
	if n, err := strconv.Atoi(compilacao); err == nil && n >= primeiraCompilacaoDoWindows11 {
		produto = strings.Replace(produto, "Windows 10", "Windows 11", 1)
	}
	nome := produto
	if versao != "" {
		nome += " " + versao
	}
	if compilacao != "" {
		nome += " (compilação " + compilacao + ")"
	}
	return nome
}
