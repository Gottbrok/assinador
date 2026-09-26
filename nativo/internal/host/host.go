// Package host é o modo padrão do programa: fala native messaging com a extensão pela entrada e
// saída padrão, uma operação por vez (pedido concorrente é `ocupado`), e sai quando a entrada
// fecha. Nada além das respostas vai para a saída padrão, e o host nunca carrega biblioteca
// PKCS#11 no próprio processo: quem as carrega são os filhos do provedor.
package host

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/Gottbrok/assinador/nativo/internal/assinatura"
	"github.com/Gottbrok/assinador/nativo/internal/bilhete"
	"github.com/Gottbrok/assinador/nativo/internal/mensagens"
	"github.com/Gottbrok/assinador/nativo/internal/origem"
	"github.com/Gottbrok/assinador/nativo/internal/protocolo"
)

// Host atende a extensão.
type Host struct {
	Entrada io.Reader
	Saida   io.Writer
	// Chamador é a extensão que o navegador disse ter lançado o programa; fora das nossas, toda
	// operação é recusada com `origem-recusada`.
	Chamador   origem.Chamador
	ChamadorOk bool
	Provedores []assinatura.Provedor
	// Chaves são as chaves de bilhete em que o programa acredita (`bilhete.ChavesDoPrograma`).
	Chaves     []bilhete.Chave
	Agora      func() time.Time
	Versao     string
	Plataforma string

	escrita sync.Mutex
}

// Executar atende até a entrada fechar. Mensagem acima do teto encerra (o fluxo não se
// ressincroniza); a entrada fechada cancela a operação em curso, o que mata os filhos.
func (h *Host) Executar(ctx context.Context) error {
	ctx, cancelar := context.WithCancel(ctx)
	defer cancelar()

	quadros := make(chan []byte)
	fim := make(chan error, 1)
	go func() {
		for {
			q, err := mensagens.LerQuadro(h.Entrada, protocolo.EntradaMaxima)
			if err != nil {
				fim <- err
				return
			}
			select {
			case quadros <- q:
			case <-ctx.Done():
				mensagens.Zerar(q)
				fim <- ctx.Err()
				return
			}
		}
	}()

	// A resposta da operação volta por este canal e é o laço que a escreve, DEPOIS de liberar o
	// `ocupado`: a extensão manda o pedido seguinte assim que recebe a resposta, e ele não pode
	// encontrar o programa ainda ocupado com a operação que já respondeu.
	var emCurso sync.WaitGroup
	respostas := make(chan protocolo.Resposta, 1)
	ocupado := false
	for {
		select {
		case q := <-quadros:
			pedido, erro := mensagens.Decodificar(q)
			mensagens.Zerar(q)
			switch {
			case erro != nil:
				h.responder(protocolo.Falha(pedido.ID, erro))
			case ocupado:
				pedido.Zerar()
				h.responder(protocolo.Falha(pedido.ID, protocolo.Novo(protocolo.Ocupado, "outra operação em curso")))
			default:
				ocupado = true
				emCurso.Go(func() { respostas <- h.atender(ctx, pedido) })
			}
		case r := <-respostas:
			ocupado = false
			h.responder(r)
		case <-ctx.Done():
			// SIGTERM ou SIGINT (o `main` os troca pelo cancelamento): a operação em curso é
			// cancelada, o que mata os filhos, e o programa sai sem esperar a entrada fechar.
			emCurso.Wait()
			return nil
		case err := <-fim:
			cancelar()
			emCurso.Wait()
			if errors.Is(err, mensagens.ErrQuadroGrande) {
				h.responder(protocolo.Falha("", protocolo.Novo(protocolo.Protocolo, "mensagem acima do limite")))
				return err
			}
			if errors.Is(err, io.EOF) || errors.Is(err, context.Canceled) {
				return nil
			}
			return err
		}
	}
}

// responder escreve uma resposta, uma de cada vez. Resposta acima do teto do navegador vira
// `interno`, para a extensão não ficar sem resposta.
func (h *Host) responder(r protocolo.Resposta) {
	b, err := json.Marshal(r)
	if err != nil || len(b) > protocolo.SaidaMaxima {
		b, _ = json.Marshal(protocolo.Falha(r.ID, protocolo.Novo(protocolo.Interno, "resposta acima do limite")))
	}
	h.escrita.Lock()
	defer h.escrita.Unlock()
	_ = mensagens.EscreverQuadro(h.Saida, b, protocolo.SaidaMaxima)
}

// atender resolve um pedido. O PIN do pedido é zerado na saída, em todo caminho, e um pânico vira
// `interno` (a extensão sempre recebe resposta).
func (h *Host) atender(ctx context.Context, p mensagens.Pedido) (r protocolo.Resposta) {
	defer p.Zerar()
	defer func() {
		if recover() != nil {
			r = protocolo.Falha(p.ID, protocolo.Novo(protocolo.Interno, "falha interna"))
		}
	}()
	if !h.ChamadorOk || !h.Chamador.Permitido() {
		return protocolo.Falha(p.ID, protocolo.Novo(protocolo.OrigemRecusada, "chamador"))
	}
	if p.Op == protocolo.OpOla {
		return protocolo.Sucesso(p.ID, protocolo.DadosDoOla{Versao: h.Versao, Protocolo: protocolo.Versao, Plataforma: h.Plataforma})
	}
	// `listar` e `diagnostico` não têm bilhete; `conferir` e `assinar` passam também pela
	// conferência do bilhete, que exige a origem exata e o padrão do emissor.
	if !origem.PermitidaSemBilhete(p.Origem) {
		return protocolo.Falha(p.ID, protocolo.Novo(protocolo.OrigemRecusada, "origem fora dos padrões"))
	}
	var dados any
	var e *protocolo.Erro
	switch p.Op {
	case protocolo.OpListar:
		dados = h.listar(ctx)
	case protocolo.OpDiagnostico:
		dados = h.diagnostico(ctx)
	case protocolo.OpConferir:
		dados, e = h.conferir(ctx, p)
	case protocolo.OpAssinar:
		dados, e = h.assinar(ctx, &p)
	default:
		e = protocolo.Novo(protocolo.Protocolo, "operação desconhecida")
	}
	if e != nil {
		return protocolo.Falha(p.ID, e)
	}
	return protocolo.Sucesso(p.ID, dados)
}
