import { describe, expect, it } from 'vitest';

import { instalarPonte, lerPedido, type CanalDoFundo, type EventoDaJanela, type JanelaDoConteudo } from '../src/conteudo';
import { PRAZOS_NA_PONTE_MS } from '../src/protocolo';
import { drenar, RelogioManual } from './apoio';

const ORIGEM = 'https://exemplo.confidata.app';

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

/** Um fundo falso atrás de cada canal: `responder` decide o que ele faz com o pedido. */
function fundoFalso(responder: (pedido: unknown, canal: { responder(m: unknown): void; cair(): void }) => void) {
  const canais: { recebidos: unknown[]; desligado: boolean }[] = [];
  const abrir = (): CanalDoFundo => {
    const estado = { recebidos: [] as unknown[], desligado: false };
    canais.push(estado);
    let aoReceber: ((m: unknown) => void) | undefined;
    let aoDesligar: (() => void) | undefined;
    return {
      enviar: (m) => {
        estado.recebidos.push(m);
        queueMicrotask(() => responder(m, { responder: (r) => aoReceber?.(r), cair: () => aoDesligar?.() }));
      },
      aoReceber: (f) => {
        aoReceber = f;
      },
      aoDesligar: (f) => {
        aoDesligar = f;
      },
      desligar: () => {
        estado.desligado = true;
      },
    };
  };
  return { abrir, canais };
}

const pedido = (extra: Record<string, unknown> = {}) => ({ canal: 'assinador:pedido', v: 1, id: 'abc-1', op: 'listar', ...extra });
const resposta = (id: string, resto: Record<string, unknown>) => ({ canal: 'assinador:resposta', v: 1, id, ...resto });

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
    expect(lerPedido(evento(undefined), janela)).toBeNull();
    expect(lerPedido(evento([pedido()]), janela)).toBeNull();
  });

  it('passa reto a carga acima de 64 KiB e o que nem vira JSON', () => {
    expect(lerPedido(evento(pedido({ dados: { bilhete: 'a'.repeat(64 * 1024) } })), janela)).toBeNull();
    const circular: Record<string, unknown> = { ...pedido() };
    circular.dados = circular;
    expect(lerPedido(evento(circular), janela)).toBeNull();
  });

  it('o que segue ao fundo é uma CÓPIA: mexer no objeto da página depois não muda o pedido', () => {
    const dados = { ref: 'r' };
    const lido = lerPedido(evento(pedido({ op: 'assinar', dados })), janela);
    dados.ref = 'trocado';
    expect(lido).toEqual({ id: 'abc-1', op: 'assinar', dados: { ref: 'r' } });
  });

  it('operação desconhecida PASSA: quem responde `protocolo` é o fundo, e a página fica sabendo', () => {
    expect(lerPedido(evento(pedido({ op: 'formatar-disco' })), janela)).toEqual({ id: 'abc-1', op: 'formatar-disco' });
  });
});

describe('instalarPonte', () => {
  it('repassa ao fundo com a origem da página, devolve a resposta só para ela, e fecha o canal', async () => {
    const { janela, postadas, disparar } = janelaFalsa();
    const f = fundoFalso((_p, c) => c.responder({ ok: true, dados: { certificados: [] } }));
    instalarPonte(janela, f.abrir);
    disparar(pedido());
    await drenar();
    expect(f.canais[0]?.recebidos).toEqual([{ tipo: 'pedido-da-pagina', id: 'abc-1', op: 'listar', origem: ORIGEM }]);
    expect(postadas).toEqual([{ mensagem: resposta('abc-1', { ok: true, dados: { certificados: [] } }), alvo: ORIGEM }]);
    expect(f.canais[0]?.desligado).toBe(true);
  });

  it('não abre canal para o que não é pedido (e ignora as próprias respostas)', async () => {
    const { janela, disparar } = janelaFalsa();
    const f = fundoFalso(() => undefined);
    instalarPonte(janela, f.abrir);
    disparar(resposta('abc-1', { ok: true, dados: {} }));
    disparar(pedido(), {});
    await drenar();
    expect(f.canais).toHaveLength(0);
  });

  it('fundo que responde sem forma é `interno`; canal que cai manda recarregar', async () => {
    const { janela, postadas, disparar } = janelaFalsa();
    let vez = 0;
    const f = fundoFalso((_p, c) => {
      vez += 1;
      if (vez === 1) c.responder(undefined);
      else c.cair();
    });
    instalarPonte(janela, f.abrir);
    disparar(pedido({ id: 'um' }));
    disparar(pedido({ id: 'dois' }));
    await drenar();
    expect(postadas.map((p) => p.mensagem)).toEqual([
      resposta('um', { ok: false, erro: { codigo: 'interno', detalhe: 'fundo sem resposta' } }),
      resposta('dois', { ok: false, erro: { codigo: 'interno', detalhe: expect.stringContaining('recarregue a página') } }),
    ]);
  });

  it('extensão atualizada com a página aberta: o `connect` lança na hora, e a página sabe na hora', async () => {
    const { janela, postadas, disparar } = janelaFalsa();
    instalarPonte(janela, () => {
      throw new Error('Extension context invalidated.');
    });
    disparar(pedido({ op: 'ola' }));
    await drenar();
    expect(postadas.map((p) => p.mensagem)).toEqual([resposta('abc-1', { ok: false, erro: { codigo: 'interno', detalhe: expect.stringContaining('recarregue a página') } })]);
  });

  it('o prazo da página é cobrado aqui: fundo calado vira `tempo-esgotado`, e a resposta atrasada não conta', async () => {
    const { janela, postadas, disparar } = janelaFalsa();
    const relogio = new RelogioManual();
    let atrasada: (() => void) | undefined;
    const f = fundoFalso((_p, c) => {
      atrasada = () => c.responder({ ok: true, dados: { extensao: { versao: '1.0.0' }, nativo: null } });
    });
    instalarPonte(janela, f.abrir, (ms, fn) => relogio.agendar(ms, fn));
    disparar(pedido({ op: 'ola' }));
    await drenar();
    expect(relogio.pendentes()).toEqual([PRAZOS_NA_PONTE_MS.ola]);
    relogio.avancar(PRAZOS_NA_PONTE_MS.ola);
    atrasada?.();
    expect(postadas.map((p) => p.mensagem)).toEqual([resposta('abc-1', { ok: false, erro: { codigo: 'tempo-esgotado', detalhe: expect.any(String) } })]);
    expect(f.canais[0]?.desligado).toBe(true);
  });

  it('cada operação tem o prazo dela; operação desconhecida usa o do listar', async () => {
    const { janela, disparar } = janelaFalsa();
    const relogio = new RelogioManual();
    instalarPonte(janela, fundoFalso(() => undefined).abrir, (ms, fn) => relogio.agendar(ms, fn));
    disparar(pedido({ id: 'a', op: 'assinar' }));
    disparar(pedido({ id: 'b', op: 'inventada' }));
    expect(relogio.pendentes()).toEqual([PRAZOS_NA_PONTE_MS.assinar, PRAZOS_NA_PONTE_MS.listar]);
  });
});
