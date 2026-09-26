import { describe, expect, it } from 'vitest';

import { criarEmbargo } from '../src/embargo';
import { criarFila } from '../src/fila';
import { RelogioManual } from './apoio';

const REGRA = { recusas: 3, janelaMs: 10_000, duracaoMs: 5_000 };

describe('embargo de endereço', () => {
  it('embarga na terceira recusa dentro da janela, e o embargo passa', () => {
    const relogio = new RelogioManual();
    const e = criarEmbargo(REGRA, relogio);
    e.registrarRecusa('a');
    e.registrarRecusa('a');
    expect(e.emEmbargo('a')).toBe(false);
    e.registrarRecusa('a');
    expect(e.emEmbargo('a')).toBe(true);
    expect(e.emEmbargo('b')).toBe(false);
    relogio.avancar(REGRA.duracaoMs);
    expect(e.emEmbargo('a')).toBe(false);
  });

  it('recusas espaçadas além da janela não somam, e aceitar zera a conta', () => {
    const relogio = new RelogioManual();
    const e = criarEmbargo(REGRA, relogio);
    e.registrarRecusa('a');
    relogio.avancar(REGRA.janelaMs);
    e.registrarRecusa('a');
    e.registrarRecusa('a');
    expect(e.emEmbargo('a')).toBe(false);
    e.registrarAceite('a');
    e.registrarRecusa('a');
    e.registrarRecusa('a');
    expect(e.emEmbargo('a')).toBe(false);
  });
});

describe('fila do dispositivo', () => {
  it('tarefa que lança vira `interno` e não trava a fila', async () => {
    const fila = criarFila(new RelogioManual(), 2);
    expect(await fila.executar(1_000, () => Promise.reject(new Error('quebrou')))).toEqual({ ok: false, erro: { codigo: 'interno', detalhe: 'quebrou' } });
    expect(await fila.executar(1_000, async () => ({ ok: true, dados: {} }))).toEqual({ ok: true, dados: {} });
  });
});
