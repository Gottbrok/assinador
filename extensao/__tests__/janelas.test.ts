import { describe, expect, it } from 'vitest';

import { criarJanelas, type ControleDeJanelas } from '../src/janelas';
import { drenar, RelogioManual } from './apoio';

function controleFalso() {
  const abertas: { url: string; janelaId: number; abaId: number }[] = [];
  const fechadas: number[] = [];
  let aoFechar: ((id: number) => void) | undefined;
  let proximo = 10;
  let segurar: ((v: { janelaId: number; abaId: number }) => void) | null = null;
  let falhar = false;
  let atrasar = false;
  const controle: ControleDeJanelas = {
    abrir(url) {
      if (falhar) return Promise.reject(new Error('sem janela'));
      proximo += 1;
      const j = { url, janelaId: proximo, abaId: proximo + 100 };
      abertas.push(j);
      if (atrasar) return new Promise((r) => (segurar = r)).then(() => j);
      return Promise.resolve(j);
    },
    async fechar(id) {
      fechadas.push(id);
    },
    aoFechar(f) {
      aoFechar = f;
    },
  };
  return {
    controle,
    abertas,
    fechadas,
    fecharPelaPessoa: (id: number) => aoFechar?.(id),
    falharAoAbrir: () => {
      falhar = true;
    },
    atrasarAbertura: () => {
      atrasar = true;
    },
    concluirAbertura: () => segurar?.({ janelaId: 0, abaId: 0 }),
  };
}

let seq = 0;
const novoId = () => `fluxo-${(seq += 1)}`;
const url = (p: string) => `chrome-extension://id/${p}`;

describe('janelas de decisão', () => {
  it('a decisão da aba do fluxo resolve e fecha a janela; outra aba não decide', async () => {
    const c = controleFalso();
    const j = criarJanelas(c.controle, url, novoId, new RelogioManual());
    const espera = j.esperar('confirmar.html', 420, 560, { documento: 'Contrato' }, 180_000);
    await drenar();
    const aberta = c.abertas[0];
    if (!aberta) throw new Error('não abriu');
    const fluxo = new URL(aberta.url).searchParams.get('fluxo') ?? '';
    expect(aberta.url).toBe(`chrome-extension://id/confirmar.html?fluxo=${fluxo}`);
    expect(j.dadosDaJanela(fluxo, 999)).toEqual({ estado: 'ausente' });
    expect(j.dadosDaJanela(fluxo, aberta.abaId)).toEqual({ estado: 'pronto', dados: { documento: 'Contrato' } });
    expect(j.decidir(fluxo, 999, { assinar: true })).toBe(false);
    expect(j.decidir(fluxo, undefined, { assinar: true })).toBe(false);
    expect(j.decidir(fluxo, aberta.abaId, { assinar: true })).toBe(true);
    expect(await espera).toEqual({ tipo: 'decisao', valor: { assinar: true } });
    expect(c.fechadas).toEqual([aberta.janelaId]);
    // Decidida, a janela não decide de novo nem entrega os dados.
    expect(j.decidir(fluxo, aberta.abaId, { assinar: true })).toBe(false);
    expect(j.dadosDaJanela(fluxo, aberta.abaId)).toEqual({ estado: 'ausente' });
  });

  it('enquanto o navegador abre a janela, os dados são `aguarde`', async () => {
    const c = controleFalso();
    c.atrasarAbertura();
    const j = criarJanelas(c.controle, url, novoId, new RelogioManual());
    void j.esperar('permitir.html', 420, 320, { host: 'demot.confidata.app' }, 25_000);
    await drenar();
    const fluxo = new URL(c.abertas[0]?.url ?? '').searchParams.get('fluxo') ?? '';
    expect(j.dadosDaJanela(fluxo, c.abertas[0]?.abaId)).toEqual({ estado: 'aguarde' });
    c.concluirAbertura();
    await drenar();
    expect(j.dadosDaJanela(fluxo, c.abertas[0]?.abaId)).toEqual({ estado: 'pronto', dados: { host: 'demot.confidata.app' } });
  });

  it('janela fechada pela pessoa é `fechada`; o prazo é `prazo` e fecha a janela', async () => {
    const c = controleFalso();
    const relogio = new RelogioManual();
    const j = criarJanelas(c.controle, url, novoId, relogio);
    const primeira = j.esperar('confirmar.html', 420, 560, {}, 180_000);
    await drenar();
    c.fecharPelaPessoa(c.abertas[0]?.janelaId ?? -1);
    expect(await primeira).toEqual({ tipo: 'fechada' });

    const segunda = j.esperar('confirmar.html', 420, 560, {}, 180_000);
    await drenar();
    relogio.avancar(179_999);
    await drenar();
    relogio.avancar(1);
    expect(await segunda).toEqual({ tipo: 'prazo' });
    expect(c.fechadas).toEqual([c.abertas[1]?.janelaId]);
  });

  it('janela que não abre é `falhou`, não `fechada`', async () => {
    const c = controleFalso();
    c.falharAoAbrir();
    const j = criarJanelas(c.controle, url, novoId, new RelogioManual());
    expect(await j.esperar('confirmar.html', 420, 560, {}, 180_000)).toEqual({ tipo: 'falhou' });
  });

  it('interromper fecha a janela aberta, e também a que ainda estava abrindo', async () => {
    const c = controleFalso();
    const j = criarJanelas(c.controle, url, novoId, new RelogioManual());
    let interromper: () => void = () => undefined;
    const aberta = j.esperar('confirmar.html', 420, 560, {}, 180_000, new Promise<void>((r) => (interromper = r)));
    await drenar();
    interromper();
    expect(await aberta).toEqual({ tipo: 'interrompida' });
    expect(c.fechadas).toEqual([c.abertas[0]?.janelaId]);

    c.atrasarAbertura();
    let interromperCedo: () => void = () => undefined;
    const abrindo = j.esperar('permitir.html', 420, 320, {}, 25_000, new Promise<void>((r) => (interromperCedo = r)));
    await drenar();
    interromperCedo();
    await drenar();
    c.concluirAbertura();
    expect(await abrindo).toEqual({ tipo: 'interrompida' });
    expect(c.fechadas).toHaveLength(2);
  });

  it('janela fechada antes de o navegador dizer que abriu conta como fechada', async () => {
    const c = controleFalso();
    c.atrasarAbertura();
    const j = criarJanelas(c.controle, url, novoId, new RelogioManual());
    const espera = j.esperar('confirmar.html', 420, 560, {}, 180_000);
    await drenar();
    c.fecharPelaPessoa(c.abertas[0]?.janelaId ?? -1);
    c.concluirAbertura();
    expect(await espera).toEqual({ tipo: 'fechada' });
  });
});
