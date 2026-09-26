// Package diagnostico monta o que o suporte recebe: o sistema, o programa, as leitoras e o ATR de
// cada cartão (com a sugestão do programa do fabricante, quando o ATR está no catálogo medido), os
// módulos PKCS#11 com o estado de cada um, os certificados (o nome mascarado) e os avisos, em frase
// (§3.5 do plano). Sem CPF: o CN ICP-Brasil é `NOME:CPF`, e os dígitos saem trocados por `*`.
//
// O mesmo relatório responde à operação `diagnostico` da extensão e ao modo `assinador
// diagnostico` do terminal.
package diagnostico

import (
	"bufio"
	"context"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"time"

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
	Agora      func() time.Time
	Versao     string
	Plataforma string
	Sistema    string
	ATRs       []catalogo.ATR
	Modulos    []catalogo.Modulo
}

// PrazoDasLeitoras é quanto o diagnóstico espera o `pcscd`. Passado o prazo, o relatório diz que
// ele não respondeu, e a consulta que ficou presa é abandonada.
var PrazoDasLeitoras = 5 * time.Second

// Coletar consulta as leitoras e os provedores e monta o relatório e o texto.
func Coletar(ctx context.Context, f Fontes) (Relatorio, string) {
	var provedores []assinatura.RelatorioDoProvedor
	for _, p := range f.Provedores {
		provedores = append(provedores, p.Diagnosticar(ctx)...)
	}
	return Montar(f, consultarComPrazo(f.Leitoras), provedores)
}

func consultarComPrazo(consultar func() pcsc.Resultado) pcsc.Resultado {
	if consultar == nil {
		return pcsc.Resultado{Estado: pcsc.EstadoSemBiblioteca, Leitoras: []pcsc.Leitora{}}
	}
	pronto := make(chan pcsc.Resultado, 1)
	go func() { pronto <- consultar() }()
	select {
	case r := <-pronto:
		return r
	case <-time.After(PrazoDasLeitoras):
		return pcsc.Resultado{Estado: pcsc.EstadoFalhou, Detalhe: "o pcscd não respondeu no prazo", Leitoras: []pcsc.Leitora{}}
	}
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
	carregados := map[string]bool{}
	for _, p := range provedores {
		r.Provedores = append(r.Provedores, p)
		if p.Estado == assinatura.EstadoCarregado {
			carregados[p.Nome] = true
		}
	}
	certificadosPorProvedor := map[string]int{}
	vistos := map[string]bool{}
	for _, p := range provedores {
		for _, c := range p.Vistos {
			if vistos[c.Ref] {
				continue
			}
			vistos[c.Ref] = true
			if resumo, ok := resumir(c, agora); ok {
				r.Certificados = append(r.Certificados, resumo)
				certificadosPorProvedor[c.RotuloDoProvedor]++
			}
		}
	}

	avisar := func(formato string, args ...any) { r.Avisos = append(r.Avisos, fmt.Sprintf(formato, args...)) }
	switch leitoras.Estado {
	case pcsc.EstadoSemBiblioteca:
		avisar("A biblioteca do PC/SC não está instalada, e sem ela nenhuma leitora aparece. Instale o pacote pcscd (no Fedora, pcsc-lite).")
	case pcsc.EstadoSemServico:
		avisar("O serviço pcscd não está rodando, e sem ele nenhuma leitora aparece. Para iniciar: sudo systemctl start pcscd.")
	case pcsc.EstadoSemLeitora:
		avisar("Nenhuma leitora de cartão foi encontrada. Confira o cabo USB da leitora ou do token.")
	case pcsc.EstadoFalhou:
		avisar("O PC/SC não respondeu como devia (%s).", leitoras.Detalhe)
	}
	algumCartao := false
	for _, l := range leitoras.Leitoras {
		item := Leitora{Nome: l.Nome, ComCartao: l.ComCartao, Mudo: l.Mudo, ATR: l.ATR}
		if l.ComCartao {
			algumCartao = true
			if m, a, ok := catalogo.ModuloDoATR(l.ATR, f.ATRs, f.Modulos); ok {
				item.Cartao, item.Sugestao = a.Cartao, m.Rotulo
				switch {
				case !carregados[m.Rotulo]:
					avisar("O cartão na leitora %s (%s) usa o %s, que não está instalado. Instale o %s.", l.Nome, a.Cartao, m.Rotulo, m.Rotulo)
				case certificadosPorProvedor[m.Rotulo] == 0:
					avisar("O %s está instalado, mas não achou certificado no cartão da leitora %s.", m.Rotulo, l.Nome)
				}
			} else if len(r.Certificados) == 0 {
				avisar("O cartão na leitora %s não foi lido por nenhum programa de cartão instalado. Ele precisa do programa do fabricante (por exemplo, o SafeSign ou o SafeNet).", l.Nome)
			}
			if l.Mudo {
				avisar("O cartão na leitora %s não responde. Tire o cartão e coloque de novo.", l.Nome)
			}
		}
		r.Leitoras = append(r.Leitoras, item)
	}
	if leitoras.Estado == pcsc.EstadoOk && !algumCartao && len(r.Certificados) == 0 {
		avisar("Nenhuma leitora tem cartão. Coloque o cartão na leitora (ou conecte o token).")
	}
	algumModulo := false
	for _, p := range provedores {
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
	saida := Certificado{Provedor: c.RotuloDoProvedor, Leitor: c.Leitor}
	x, err := x509.ParseCertificate(c.DER)
	if err != nil {
		saida.Titular, saida.Situacao = "(ilegível)", SituacaoIlegivel
		return saida, true
	}
	if x.IsCA {
		return Certificado{}, false
	}
	saida.Titular = assinatura.Mascarar(x.Subject.CommonName)
	saida.Emissor = x.Issuer.CommonName
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
	pcsc.EstadoSemServico:    "serviço pcscd parado",
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

// SistemaOperacional é o nome do sistema para o relatório (o PRETTY_NAME do os-release, no
// Linux), ou vazio.
func SistemaOperacional() string {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return ""
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		if valor, ok := strings.CutPrefix(s.Text(), "PRETTY_NAME="); ok {
			return strings.Trim(valor, `"'`)
		}
	}
	return ""
}
