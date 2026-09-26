/**
 * O EMBARGO de um endereço que insiste em abrir janela. Cada janela recusada (negada, fechada ou
 * vencida) conta; `recusas` delas dentro de `janelaMs` embargam o endereço por `duracaoMs`, e nesse
 * tempo o pedido é recusado sem janela nenhuma. Uma decisão positiva zera a conta.
 *
 * Sem isso, uma página permitida e comprometida abre janela atrás de janela, com o foco, até a pessoa
 * ceder. Mora na memória do fundo: se o navegador o reinicia, a conta recomeça, e isso basta, porque
 * o que se quer é quebrar a rajada, não punir ninguém.
 */

import type { Relogio } from './nativo';

export interface RegraDoEmbargo {
  recusas: number;
  janelaMs: number;
  duracaoMs: number;
}

export interface Embargo {
  emEmbargo(origem: string): boolean;
  registrarRecusa(origem: string): void;
  registrarAceite(origem: string): void;
}

export function criarEmbargo(regra: RegraDoEmbargo, relogio: Pick<Relogio, 'agora'>): Embargo {
  const recusas = new Map<string, number[]>();
  const ate = new Map<string, number>();

  return {
    emEmbargo(origem) {
      const fim = ate.get(origem);
      if (fim === undefined) return false;
      if (relogio.agora() < fim) return true;
      ate.delete(origem);
      return false;
    },
    registrarRecusa(origem) {
      const agora = relogio.agora();
      const recentes = (recusas.get(origem) ?? []).filter((t) => agora - t < regra.janelaMs);
      recentes.push(agora);
      if (recentes.length >= regra.recusas) {
        ate.set(origem, agora + regra.duracaoMs);
        recusas.delete(origem);
        return;
      }
      recusas.set(origem, recentes);
    },
    registrarAceite(origem) {
      recusas.delete(origem);
    },
  };
}
