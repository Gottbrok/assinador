package host

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/Gottbrok/assinador/nativo/internal/assinatura"
	"github.com/Gottbrok/assinador/nativo/internal/bilhete"
	"github.com/Gottbrok/assinador/nativo/internal/catalogo"
	"github.com/Gottbrok/assinador/nativo/internal/diagnostico"
	"github.com/Gottbrok/assinador/nativo/internal/mensagens"
	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
)

// Avisos da lista, em frase para a pessoa.
const (
	AvisoAlgoritmo = "Um certificado com chave que não é RSA foi ignorado: o Assinador ainda não assina com ela."
	AvisoIlegivel  = "Um certificado do dispositivo não pôde ser lido e foi ignorado."
)

// achado é um certificado com o provedor que o achou.
type achado struct {
	certificado assinatura.Certificado
	provedor    assinatura.Provedor
}

// listarTodos junta os certificados de todos os provedores, sem repetir `ref` (fica o primeiro). A
// `ref` é RECALCULADA do DER aqui, e não aceita do provedor: é contra ela que o `cer` do bilhete é
// conferido, e cada provedor (o PKCS#11 hoje, o do Windows na F6a) a monta do seu jeito.
func (h *Host) listarTodos(ctx context.Context) ([]achado, []string) {
	var achados []achado
	var avisos []string
	vistos := map[string]bool{}
	for _, p := range h.Provedores {
		certs, av := p.Listar(ctx)
		avisos = append(avisos, av...)
		for _, c := range certs {
			if c.Ref != assinatura.Ref(c.DER) || vistos[c.Ref] {
				continue
			}
			vistos[c.Ref] = true
			achados = append(achados, achado{certificado: c, provedor: p})
		}
	}
	return achados, avisos
}

func (h *Host) listar(ctx context.Context) protocolo.DadosDoListar {
	achados, avisos := h.listarTodos(ctx)
	saida := protocolo.DadosDoListar{Certificados: []protocolo.CertificadoListado{}, Avisos: []string{}}
	algoritmo, ilegivel := false, false
	for _, a := range achados {
		c := a.certificado
		x, err := x509.ParseCertificate(c.DER)
		if err != nil {
			ilegivel = true
			continue
		}
		if ok, motivo := assinatura.Listavel(x); !ok {
			algoritmo = algoritmo || motivo == assinatura.ForaDaListaAlgoritmo
			continue
		}
		saida.Certificados = append(saida.Certificados, protocolo.CertificadoListado{
			Ref:              c.Ref,
			DER:              base64.StdEncoding.EncodeToString(c.DER),
			Provedor:         c.Provedor,
			RotuloDoProvedor: c.RotuloDoProvedor,
			Leitor:           c.Leitor,
			ExigePin:         c.ExigePin,
			EstadoDoPin:      c.EstadoDoPin,
		})
	}
	saida.Avisos = append(saida.Avisos, avisos...)
	if algoritmo {
		saida.Avisos = append(saida.Avisos, AvisoAlgoritmo)
	}
	if ilegivel {
		saida.Avisos = append(saida.Avisos, AvisoIlegivel)
	}
	return saida
}

// prepararAto confere o bilhete ANTES de tocar em qualquer módulo (pedido forjado não carrega
// biblioteca de fabricante), acha o certificado de `ref` e confere que ele pode assinar agora.
// O `cer` do bilhete é conferido contra a `ref`, que é o SHA-256 do DER: o provedor calculou a ref
// do DER que leu, e o pai a conferiu.
func (h *Host) prepararAto(ctx context.Context, p mensagens.Pedido) (achado, *x509.Certificate, bilhete.Bilhete, *protocolo.Erro) {
	agora := h.Agora()
	r := bilhete.Conferir(p.Bilhete, h.Chaves, bilhete.Entrada{Origem: p.Origem, Digest: p.Digest, CertificadoSha256: p.Ref, Agora: agora})
	if !r.OK {
		return achado{}, nil, bilhete.Bilhete{}, r.Erro()
	}
	achados, _ := h.listarTodos(ctx)
	for _, a := range achados {
		if a.certificado.Ref != p.Ref {
			continue
		}
		x, err := x509.ParseCertificate(a.certificado.DER)
		if err != nil {
			return achado{}, nil, bilhete.Bilhete{}, protocolo.Novo(protocolo.CertificadoNaoEncontrado, "certificado ilegível")
		}
		if e := assinatura.Assinavel(x, agora); e != nil {
			return achado{}, nil, bilhete.Bilhete{}, e
		}
		return a, x, r.Bilhete, nil
	}
	return achado{}, nil, bilhete.Bilhete{}, protocolo.Novo(protocolo.CertificadoNaoEncontrado, "nenhum dispositivo presente tem o certificado")
}

func (h *Host) conferir(ctx context.Context, p mensagens.Pedido) (protocolo.DadosDoConferir, *protocolo.Erro) {
	a, x, b, e := h.prepararAto(ctx, p)
	if e != nil {
		return protocolo.DadosDoConferir{}, e
	}
	return protocolo.DadosDoConferir{
		Emissor:     b.Iss,
		Organizacao: b.Org,
		Documento:   b.Doc,
		Finalidade:  b.Fin,
		ExpiraEm:    time.Unix(b.Exp, 0).UTC().Format(time.RFC3339),
		Certificado: protocolo.CertificadoParaConferir{
			Assunto:     assinatura.Mascarar(x.Subject.CommonName),
			Emissor:     x.Issuer.CommonName,
			ValidoAte:   x.NotAfter.UTC().Format(time.RFC3339),
			ExigePin:    a.certificado.ExigePin,
			EstadoDoPin: a.certificado.EstadoDoPin,
		},
	}, nil
}

// assinar confere tudo de novo (o programa não guarda estado entre `conferir` e `assinar`), manda
// o PIN ao provedor só se o token o exige, e confere a assinatura devolvida contra o certificado.
func (h *Host) assinar(ctx context.Context, p *mensagens.Pedido) (protocolo.DadosDoAssinar, *protocolo.Erro) {
	a, x, _, e := h.prepararAto(ctx, *p)
	if e != nil {
		return protocolo.DadosDoAssinar{}, e
	}
	var digest [32]byte
	if n, err := hex.Decode(digest[:], []byte(p.Digest)); err != nil || n != len(digest) {
		return protocolo.DadosDoAssinar{}, protocolo.Novo(protocolo.Protocolo, "digest fora do formato")
	}
	var pin []byte
	if a.certificado.ExigePin {
		// PIN vazio nunca vai ao cartão: há middleware que conta como tentativa errada.
		if !p.TemPin || len(p.Pin) == 0 {
			return protocolo.DadosDoAssinar{}, protocolo.Novo(protocolo.Protocolo, "o dispositivo exige PIN e ele não veio")
		}
		pin = p.Pin
	}
	assinada, err := a.provedor.Assinar(ctx, a.certificado, digest, pin)
	p.Zerar()
	if err != nil {
		var pe *protocolo.Erro
		if errors.As(err, &pe) {
			return protocolo.DadosDoAssinar{}, pe
		}
		return protocolo.DadosDoAssinar{}, protocolo.Novo(protocolo.Interno, err.Error())
	}
	if err := assinatura.ConferirAssinatura(x, digest, assinada); err != nil {
		return protocolo.DadosDoAssinar{}, protocolo.Novo(protocolo.Interno, "a assinatura do dispositivo não confere com o certificado")
	}
	return protocolo.DadosDoAssinar{Assinatura: base64.StdEncoding.EncodeToString(assinada)}, nil
}

// diagnostico é o relatório do pacote `diagnostico`: o mesmo que o modo `assinador diagnostico`
// imprime no terminal.
func (h *Host) diagnostico(ctx context.Context) protocolo.DadosDoDiagnostico {
	r, texto := diagnostico.Coletar(ctx, diagnostico.Fontes{
		Provedores:          h.Provedores,
		Leitoras:            h.Leitoras,
		ServicoDePropagacao: h.ServicoDePropagacao,
		Agora:               h.Agora,
		Versao:              h.Versao,
		Plataforma:          h.Plataforma,
		Sistema:             h.Sistema,
		ATRs:                catalogo.ATRs,
		Modulos:             catalogo.Modulos,
	})
	return protocolo.DadosDoDiagnostico{Relatorio: r, Texto: texto}
}
