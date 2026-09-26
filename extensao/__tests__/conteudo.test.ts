import { describe, expect, it, vi } from 'vitest';

import { instalarPonte, lerPedido, type EventoDaJanela, type JanelaDoConteudo, type PedidoParaOFundo } from '../src/conteudo';
import { drenar } from './apoio';

const ORIGEM = 'https://demot.confidata.app';

function janelaFalsa() {
  const ouvintes: ((e: EventoDaJanela) => void)[] = [];
  const postadas: { mensagem: unknown; alvo: string }[] = [];
  const janela: JanelaDoConteudo = {
    location: { origin: ORIGEM },
    addEventListener: (_tipo, f) => {
      ouvintes.push(f);
    },
    postMessage: (mensagem, alvo) => {
      postadas.push({ mensagem, alvo });
    },
  };
  const disparar = (data: unknown, source: unknown = janela, origin = ORIGEM) => {
    for (const f of ouvintes) f({ data, source, origin });
  };
  return { janela, postadas, disparar };
}

const pedido = (extra: Record<string, unknown> = {}) => ({ canal: 'assinador:pedido', v: 1, id: 'abc-1', op: 'listar', ...extra });

describe('lerPedido', () => {
  const { janela } = janelaFalsa();
  const evento = (data: unknown, source: unknown = janela, origin = ORIGEM): EventoDaJanela => ({ data, source, origin });

  it('aceita o pedido da própria janela, com canal, versão e id certos', () => {
    expect(lerPedido(evento(pedido()), janela)).toEqual({ id: 'abc-1', op: 'listar' });
    expect(lerPedido(evento(pedido({ op: 'assinar', dados: { ref: 'r' } })), janela)).toEqual({ id: 'abc-1', op: 'assinar', dados: { ref: 'r' } });
  });

  it('passa reto a mensagem de outra fonte (iframe, outra janela) ou de outra origem', () => {
    expect(lerPedido(evento(pedido(), {}), janela)).toBeNull();
    expect(lerPedido(evento(pedido(), janela, 'https://evil.com'), janela)).toBeNull();
  });

  it('passa reto canal, versão ou id errados, chave a mais e resposta', () => {
    expect(lerPedido(evento(pedido({ canal: 'assinador:resposta' })), janela)).toBeNull();
    expect(lerPedido(evento(pedido({ v: 2 })), janela)).toBeNull();
    expect(lerPedido(evento(pedido({ id: 'com espaço' })), janela)).toBeNull();
    expect(lerPedido(evento(pedido({ id: 'x'.repeat(65) })), janela)).toBeNull();
    expect(lerPedido(evento(pedido({ origem: ORIGEM })), janela)).toBeNull();
    expect(lerPedido(evento('texto'), janela)).toBeNull();
    expect(lerPedido(evento(null), janela)).toBeNull();
    expect(lerPedido(evento([pedido()]), janela)).toBeNull();
  });

  it('passa reto a carga acima de 64 KiB', () => {
    expect(lerPedido(evento(pedido({ dados: { bilhete: 'a'.repeat(64 * 1024) } })), janela)).toBeNull();
  });

  it('operação desconhecida PASSA: quem responde `protocolo` é o fundo, e a página fica sabendo', () => {
    expect(lerPedido(evento(pedido({ op: 'formatar-disco' })), janela)).toEqual({ id: 'abc-1', op: 'formatar-disco' });
  });
});

describe('instalarPonte', () => {
  it('repassa ao fundo com a origem da página e devolve a resposta só para ela', async () => {
    const { janela, postadas, disparar } = janelaFalsa();
    const recebidos: PedidoParaOFundo[] = [];
    instalarPonte(janela, async (p) => {
      recebidos.push(p);
      return { ok: true, dados: { certificados: [] } };
    });
    disparar(pedido());
    await drenar();
    expect(recebidos).toEqual([{ tipo: 'pedido-da-pagina', id: 'abc-1', op: 'listar', origem: ORIGEM }]);
    expect(postadas).toEqual([{ mensagem: { canal: 'assinador:resposta', v: 1, id: 'abc-1', ok: true, dados: { certificados: [] } }, alvo: ORIGEM }]);
  });

  it('não repassa o que não é pedido (e ignora as próprias respostas)', async () => {
    const { janela, disparar } = janelaFalsa();
    const enviar = vi.fn(async () => ({ ok: true, dados: {} }));
    instalarPonte(janela, enviar);
    disparar({ canal: 'assinador:resposta', v: 1, id: 'abc-1', ok: true, dados: {} });
    disparar(pedido(), {});
    await drenar();
    expect(enviar).not.toHaveBeenCalled();
  });

  it('fundo que responde sem forma vira `interno`, e fundo que some manda recarregar', async () => {
    const { janela, postadas, disparar } = janelaFalsa();
    let vez = 0;
    instalarPonte(janela, async () => {
      vez += 1;
      if (vez === 1) return undefined;
      throw new Error('Extension context invalidated.');
    });
    disparar(pedido({ id: 'um' }));
    disparar(pedido({ id: 'dois' }));
    await drenar();
    expect(postadas.map((p) => p.mensagem)).toEqual([
      { canal: 'assinador:resposta', v: 1, id: 'um', ok: false, erro: { codigo: 'interno', detalhe: 'fundo sem resposta' } },
      { canal: 'assinador:resposta', v: 1, id: 'dois', ok: false, erro: { codigo: 'interno', detalhe: 'a extensão foi atualizada: recarregue a página' } },
    ]);
  });
});
