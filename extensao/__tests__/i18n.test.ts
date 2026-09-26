import { readdirSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { describe, expect, it } from 'vitest';

import { dataParaLer } from '../src/i18n';

const raiz = resolve(__dirname, '..');
type Mensagens = Record<string, { message: string; placeholders?: Record<string, { content: string }> }>;
const ler = (idioma: string): Mensagens => JSON.parse(readFileSync(resolve(raiz, '_locales', idioma, 'messages.json'), 'utf8')) as Mensagens;
const ptBR = ler('pt_BR');
const es = ler('es');

/** As chaves que o código pede: `t('chave')`, `t('chave', ...)`, `t(cond ? 'a' : 'b')` e as que um helper devolve. */
function chavesDoCodigo(): Set<string> {
  const chaves = new Set<string>();
  for (const arquivo of readdirSync(resolve(raiz, 'src'))) {
    const codigo = readFileSync(resolve(raiz, 'src', arquivo), 'utf8');
    for (const m of codigo.matchAll(/\bt\(\s*'([A-Za-z0-9_]+)'/g)) chaves.add(m[1] as string);
    for (const m of codigo.matchAll(/\bt\([^)]*\?\s*'([A-Za-z0-9_]+)'\s*:\s*'([A-Za-z0-9_]+)'/g)) {
      chaves.add(m[1] as string);
      chaves.add(m[2] as string);
    }
    for (const m of codigo.matchAll(/return '((?:aviso|finalidade)[A-Za-z0-9_]+)'/g)) chaves.add(m[1] as string);
  }
  return chaves;
}

function chavesDasPaginas(): Set<string> {
  const chaves = new Set<string>();
  for (const arquivo of readdirSync(resolve(raiz, 'paginas')).filter((a) => a.endsWith('.html'))) {
    const html = readFileSync(resolve(raiz, 'paginas', arquivo), 'utf8');
    for (const m of html.matchAll(/data-i18n(?:-title|-aria-label)?="([A-Za-z0-9_]+)"/g)) chaves.add(m[1] as string);
  }
  return chaves;
}

describe('textos da extensão', () => {
  it('pt_BR e es têm as mesmas chaves e os mesmos marcadores', () => {
    expect(Object.keys(es).sort()).toEqual(Object.keys(ptBR).sort());
    for (const [chave, m] of Object.entries(ptBR)) {
      const outro = es[chave];
      expect(Object.keys(outro?.placeholders ?? {}).sort(), chave).toEqual(Object.keys(m.placeholders ?? {}).sort());
      for (const nome of Object.keys(m.placeholders ?? {})) expect(outro?.message, chave).toContain(`$${nome.toUpperCase()}$`);
      for (const nome of Object.keys(m.placeholders ?? {})) expect(m.message, chave).toContain(`$${nome.toUpperCase()}$`);
    }
  });

  it('toda chave que as páginas e o código pedem existe, e nenhuma sobra', () => {
    const usadas = new Set([...chavesDoCodigo(), ...chavesDasPaginas(), 'nomeDaExtensao', 'descricaoDaExtensao']);
    const faltam = [...usadas].filter((c) => !(c in ptBR));
    expect(faltam).toEqual([]);
    const sobram = Object.keys(ptBR).filter((c) => !usadas.has(c));
    expect(sobram).toEqual([]);
  });

  it('nome e descrição cabem nos limites da loja, e nenhum texto tem travessão', () => {
    for (const m of [ptBR, es]) {
      expect(m.nomeDaExtensao?.message.length).toBeLessThanOrEqual(75);
      expect(m.descricaoDaExtensao?.message.length).toBeLessThanOrEqual(132);
      for (const [chave, { message }] of Object.entries(m)) expect(message, chave).not.toMatch(/[–—]/);
    }
  });

  it('o espanhol não afirma o documento brasileiro', () => {
    for (const [chave, { message }] of Object.entries(es)) expect(message, chave).not.toMatch(/\bCPF\b/);
  });
});

describe('dataParaLer', () => {
  it('validade em UTC, no idioma pedido', () => {
    expect(dataParaLer('2027-09-26T00:00:00Z', 'pt-BR')).toBe('26/09/2027');
    expect(dataParaLer('2027-09-26T23:30:00Z', 'pt-BR')).toBe('26/09/2027');
    expect(dataParaLer('2027-09-26T00:00:00Z', 'es')).toBe('26/09/2027');
  });

  it('texto sem forma volta como veio', () => {
    expect(dataParaLer('amanhã', 'pt-BR')).toBe('amanhã');
  });
});
