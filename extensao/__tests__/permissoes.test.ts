import { describe, expect, it } from 'vitest';

import { CHAVE_DAS_PERMISSOES, listarPermissoes, permitido, permitir, revogar } from '../src/permissoes';
import { armazenamentoEmMemoria } from './apoio';

describe('permissões por endereço', () => {
  it('endereço novo não tem permissão; permitido, tem; revogado, volta a não ter', async () => {
    const a = armazenamentoEmMemoria();
    expect(await permitido(a, 'https://demot.confidata.app')).toBe(false);
    await permitir(a, 'https://demot.confidata.app', new Date('2026-09-26T12:00:00Z'));
    expect(await permitido(a, 'https://demot.confidata.app')).toBe(true);
    expect(await listarPermissoes(a)).toEqual([{ origem: 'https://demot.confidata.app', desde: '2026-09-26T12:00:00.000Z' }]);
    await revogar(a, 'https://demot.confidata.app');
    expect(await permitido(a, 'https://demot.confidata.app')).toBe(false);
    expect(await listarPermissoes(a)).toEqual([]);
  });

  it('a permissão é da ORIGEM exata: outra organização não herda', async () => {
    const a = armazenamentoEmMemoria();
    await permitir(a, 'https://demot.confidata.app');
    expect(await permitido(a, 'https://outra.confidata.app')).toBe(false);
    expect(await permitido(a, 'http://demot.confidata.app')).toBe(false);
  });

  it('entrada sem forma nunca vira permissão, e nome herdado do protótipo também não', async () => {
    const a = armazenamentoEmMemoria({ [CHAVE_DAS_PERMISSOES]: { 'https://x.confidata.app': true, 'https://y.confidata.app': { desde: 1 } } });
    expect(await permitido(a, 'https://x.confidata.app')).toBe(false);
    expect(await permitido(a, 'https://y.confidata.app')).toBe(false);
    expect(await permitido(a, 'toString')).toBe(false);
    expect(await permitido(a, '__proto__')).toBe(false);
    const lixo = armazenamentoEmMemoria({ [CHAVE_DAS_PERMISSOES]: ['https://x.confidata.app'] });
    expect(await permitido(lixo, 'https://x.confidata.app')).toBe(false);
  });

  it('lista em ordem, e revogar o que não existe não escreve', async () => {
    const a = armazenamentoEmMemoria();
    await permitir(a, 'https://z.confidata.app');
    await permitir(a, 'https://a.confidata.app');
    expect((await listarPermissoes(a)).map((p) => p.origem)).toEqual(['https://a.confidata.app', 'https://z.confidata.app']);
    const antes = JSON.stringify(a.dados);
    await revogar(a, 'https://nao.confidata.app');
    expect(JSON.stringify(a.dados)).toBe(antes);
  });
});
