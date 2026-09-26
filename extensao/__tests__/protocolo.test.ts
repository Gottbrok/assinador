import { describe, expect, it } from 'vitest';

import fixture from '../../protocolo/fixtures/bilhete/protocolo.json';
import {
  CODIGOS_DE_ERRO,
  ehCodigoDeErro,
  PRAZO_DA_DECISAO_MS,
  PRAZO_DA_PERMISSAO_MS,
  PRAZOS_DO_FLUXO_MS,
  PRAZOS_DO_PROGRAMA_MS,
  PROTOCOLO,
  RESERVA_DEPOIS_DA_CONFIRMACAO_MS,
  TAMANHO_MAXIMO_DO_BILHETE,
  type CodigoDeErro,
} from '../src/protocolo';

/**
 * `PRAZOS_MS` da biblioteca (`assinadorProtocolo.ts`), o prazo com que a PÁGINA desiste. A extensão
 * tem de responder antes, sempre.
 */
const PRAZOS_DA_PAGINA_MS = { ola: 1_500, listar: 30_000, diagnostico: 30_000, assinar: 240_000 };

// Um `Record` exaustivo: código novo no tipo sem entrada aqui não compila, e o teste compara com a fixture.
const TODOS: Record<CodigoDeErro, true> = {
  'origem-recusada': true,
  'bilhete-invalido': true,
  'bilhete-expirado': true,
  relogio: true,
  'digest-divergente': true,
  'certificado-divergente': true,
  'certificado-nao-encontrado': true,
  'chave-ausente': true,
  'algoritmo-nao-suportado': true,
  'permissao-negada': true,
  'pin-incorreto': true,
  'token-bloqueado': true,
  cancelado: true,
  'tempo-esgotado': true,
  ocupado: true,
  'nativo-ausente': true,
  'nativo-desatualizado': true,
  'modulo-falhou': true,
  protocolo: true,
  interno: true,
};

describe('vocabulário do protocolo', () => {
  it('os códigos de erro do tipo são exatamente os da fixture da biblioteca', () => {
    expect(Object.keys(TODOS).sort()).toEqual([...fixture.codigosDeErro].sort());
    expect([...CODIGOS_DE_ERRO].sort()).toEqual([...fixture.codigosDeErro].sort());
    expect(ehCodigoDeErro('ocupado')).toBe(true);
    expect(ehCodigoDeErro('inventado')).toBe(false);
  });

  it('versão do protocolo e teto do bilhete vêm da fixture', () => {
    expect(PROTOCOLO).toBe(fixture.protocoloDaPagina);
    expect(TAMANHO_MAXIMO_DO_BILHETE).toBe(fixture.tamanhoMaximoDoBilhete);
  });

  it('o orçamento de cada operação fica abaixo do prazo da página', () => {
    for (const op of ['ola', 'listar', 'diagnostico', 'assinar'] as const) {
      expect(PRAZOS_DO_FLUXO_MS[op]).toBeLessThan(PRAZOS_DA_PAGINA_MS[op]);
    }
    // Um pedido só ao programa cabe inteiro no orçamento das operações de um passo.
    expect(PRAZOS_DO_PROGRAMA_MS.listar).toBeLessThanOrEqual(PRAZOS_DO_FLUXO_MS.listar);
    expect(PRAZOS_DO_PROGRAMA_MS.diagnostico).toBeLessThanOrEqual(PRAZOS_DO_FLUXO_MS.diagnostico);
    // A janela de confirmação inteira cabe no orçamento do assinar, com a reserva para o cartão.
    expect(PRAZO_DA_DECISAO_MS + RESERVA_DEPOIS_DA_CONFIRMACAO_MS).toBeLessThan(PRAZOS_DO_FLUXO_MS.assinar);
    expect(PRAZO_DA_PERMISSAO_MS).toBeLessThan(PRAZOS_DO_FLUXO_MS.listar);
  });
});
