import { describe, expect, it } from 'vitest';

import { ehChaveDePermissao, listarPermissoes, permitido, permitir, PREFIXO_DA_PERMISSAO, revogar } from '../src/permissoes';
import { armazenamentoEmMemoria } from './apoio';

const ORIGEM = 'https://exemplo.confidata.app';

describe('permissões por endereço', () => {
  it('endereço novo não tem permissão; permitido, tem; revogado, volta a não ter', async () => {
    const a = armazenamentoEmMemoria();
    expect(await permitido(a, ORIGEM)).toBe(false);
    await permitir(a, ORIGEM, new Date('2026-09-26T12:00:00Z'));
    expect(await permitido(a, ORIGEM)).toBe(true);
    expect(await listarPermissoes(a)).toEqual([{ origem: ORIGEM, desde: '2026-09-26T12:00:00.000Z' }]);
    await revogar(a, ORIGEM);
    expect(await permitido(a, ORIGEM)).toBe(false);
    expect(await listarPermissoes(a)).toEqual([]);
  });

  it('a permissão é da ORIGEM exata: outra organização não herda', async () => {
    const a = armazenamentoEmMemoria();
    await permitir(a, ORIGEM);
    expect(await permitido(a, 'https://outra.confidata.app')).toBe(false);
    expect(await permitido(a, 'http://exemplo.confidata.app')).toBe(false);
  });

  it('entrada sem forma nunca vira permissão', async () => {
    const a = armazenamentoEmMemoria({
      [`${PREFIXO_DA_PERMISSAO}https://x.confidata.app`]: true,
      [`${PREFIXO_DA_PERMISSAO}https://y.confidata.app`]: { desde: 1 },
      [`${PREFIXO_DA_PERMISSAO}https://z.confidata.app`]: ['2026'],
    });
    for (const o of ['https://x.confidata.app', 'https://y.confidata.app', 'https://z.confidata.app']) expect(await permitido(a, o)).toBe(false);
    expect(await listarPermissoes(a)).toEqual([]);
  });

  it('nome de propriedade herdada não é permissão, nem na leitura nem na lista', async () => {
    const a = armazenamentoEmMemoria();
    expect(await permitido(a, 'toString')).toBe(false);
    expect(await permitido(a, '__proto__')).toBe(false);
  });

  it('gravar uma origem e apagar outra não se desfazem (uma chave por origem)', async () => {
    const a = armazenamentoEmMemoria();
    await permitir(a, 'https://a.confidata.app');
    // As duas escritas em voo ao mesmo tempo, como o fundo e as opções fariam.
    await Promise.all([permitir(a, 'https://b.confidata.app'), revogar(a, 'https://a.confidata.app')]);
    expect((await listarPermissoes(a)).map((p) => p.origem)).toEqual(['https://b.confidata.app']);
  });

  it('lista em ordem e ignora chave que não é de permissão', async () => {
    const a = armazenamentoEmMemoria({ outraCoisa: { desde: '2026' } });
    await permitir(a, 'https://z.confidata.app');
    await permitir(a, 'https://a.confidata.app');
    expect((await listarPermissoes(a)).map((p) => p.origem)).toEqual(['https://a.confidata.app', 'https://z.confidata.app']);
    expect(ehChaveDePermissao(`${PREFIXO_DA_PERMISSAO}x`)).toBe(true);
    expect(ehChaveDePermissao('outraCoisa')).toBe(false);
  });
});
