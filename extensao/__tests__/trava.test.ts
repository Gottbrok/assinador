import { describe, expect, it } from 'vitest';

import { ATRASO_DE_SEGURANCA_MS, travarAteVer, type AmbienteDaTrava } from '../src/janela';
import { RelogioManual } from './apoio';

/** Uma janela falsa: o teste diz se ela está visível e com foco, e avisa a trava da mudança. */
function janelaFalsa(inicial: boolean) {
  const relogio = new RelogioManual();
  let visivelEComFoco = inicial;
  let ouvinte: () => void = () => undefined;
  const amb: AmbienteDaTrava = {
    visivelEComFoco: () => visivelEComFoco,
    aoMudar: (f) => {
      ouvinte = f;
    },
    agendar: (ms, f) => relogio.agendar(ms, f),
  };
  const mudar = (v: boolean) => {
    visivelEComFoco = v;
    ouvinte();
  };
  return { amb, relogio, mudar };
}

function montar(inicial: boolean) {
  const j = janelaFalsa(inicial);
  const estados: boolean[] = [];
  let focos = 0;
  const trava = travarAteVer(j.amb, (travado) => estados.push(travado), () => {
    focos += 1;
  });
  return { ...j, estados, trava, focos: () => focos };
}

describe('trava dos botões das janelas', () => {
  it('nasce travada e destrava só depois do atraso, com a janela visível e com foco', () => {
    const t = montar(true);
    expect(t.estados).toEqual([true]);
    t.relogio.avancar(ATRASO_DE_SEGURANCA_MS - 1);
    expect(t.estados).toEqual([true]);
    t.relogio.avancar(1);
    expect(t.estados).toEqual([true, false]);
    expect(t.focos()).toBe(1);
  });

  it('janela que abre sem foco não conta o atraso até ganhar o foco', () => {
    const t = montar(false);
    t.relogio.avancar(10 * ATRASO_DE_SEGURANCA_MS);
    expect(t.estados).toEqual([true]);
    t.mudar(true);
    t.relogio.avancar(ATRASO_DE_SEGURANCA_MS);
    expect(t.estados).toEqual([true, false]);
  });

  it('perder o foco trava de novo e recomeça a contagem; o foco inicial vai só no primeiro destrave', () => {
    const t = montar(true);
    t.relogio.avancar(ATRASO_DE_SEGURANCA_MS);
    expect(t.estados).toEqual([true, false]);
    // Uma janela da página por cima rouba o foco e o devolve no primeiro clique de um clique duplo.
    t.mudar(false);
    expect(t.estados).toEqual([true, false, true]);
    t.mudar(true);
    t.relogio.avancar(ATRASO_DE_SEGURANCA_MS - 1);
    expect(t.estados).toEqual([true, false, true]);
    t.relogio.avancar(1);
    expect(t.estados).toEqual([true, false, true, false]);
    expect(t.focos()).toBe(1);
  });

  it('perder o foco NO MEIO da contagem cancela a contagem', () => {
    const t = montar(true);
    t.relogio.avancar(ATRASO_DE_SEGURANCA_MS / 2);
    t.mudar(false);
    t.relogio.avancar(ATRASO_DE_SEGURANCA_MS);
    expect(t.estados).toEqual([true]);
  });

  it('encerrada (a pessoa decidiu), a trava não mexe mais nos botões', () => {
    const t = montar(true);
    t.trava.encerrar();
    t.relogio.avancar(ATRASO_DE_SEGURANCA_MS);
    t.mudar(false);
    t.mudar(true);
    t.relogio.avancar(ATRASO_DE_SEGURANCA_MS);
    expect(t.estados).toEqual([true]);
  });
});
