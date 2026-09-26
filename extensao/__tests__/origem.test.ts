import { describe, expect, it } from 'vitest';

import { hostDaOrigem, origemAceita, origemDoRemetente } from '../src/origem';

describe('origemAceita', () => {
  it('aceita as organizações do Confidata e o ushield', () => {
    expect(origemAceita('https://demot.confidata.app', false)).toBe(true);
    expect(origemAceita('https://ushield.app', false)).toBe(true);
  });

  it('recusa o que só parece', () => {
    for (const o of [
      'https://confidata.app',
      'https://a.b.confidata.app',
      'http://demot.confidata.app',
      'https://demot.confidata.app.evil.com',
      'https://demot.confidata.app:8443',
      'https://ushield.app.evil.com',
      'https://www.ushield.app',
      'https://evilconfidata.app',
      'https://DEMOT.confidata.app',
      '',
      'null',
    ]) {
      expect(origemAceita(o, false), o).toBe(false);
      expect(origemAceita(o, true), o).toBe(false);
    }
  });

  it('localhost só no build de desenvolvimento, com porta ou subdomínio', () => {
    for (const o of ['http://localhost', 'http://localhost:3000', 'http://demot.localhost:3000']) {
      expect(origemAceita(o, false), o).toBe(false);
      expect(origemAceita(o, true), o).toBe(true);
    }
    expect(origemAceita('https://localhost:3000', true)).toBe(false);
    expect(origemAceita('http://localhost.evil.com', true)).toBe(false);
  });

  it('recusa origem longa demais', () => {
    expect(origemAceita(`https://${'a'.repeat(300)}.confidata.app`, false)).toBe(false);
  });
});

describe('origemDoRemetente', () => {
  it('prefere o sender.origin (Chrome e Edge)', () => {
    expect(origemDoRemetente({ origin: 'https://demot.confidata.app', url: 'https://outra.confidata.app/x' })).toBe('https://demot.confidata.app');
  });

  it('no Firefox, sem sender.origin, usa a origem de sender.url', () => {
    expect(origemDoRemetente({ url: 'https://demot.confidata.app/assinar/abc?x=1#y' })).toBe('https://demot.confidata.app');
  });

  it('origem opaca ou ausente é null', () => {
    expect(origemDoRemetente({ origin: 'null' })).toBeNull();
    // Origem presente e opaca nunca recua à URL (documento em sandbox com URL de host permitido).
    expect(origemDoRemetente({ origin: 'null', url: 'https://demot.confidata.app/x' })).toBeNull();
    expect(origemDoRemetente({ origin: '', url: 'https://demot.confidata.app/x' })).toBeNull();
    expect(origemDoRemetente({ url: 'data:text/html,oi' })).toBeNull();
    expect(origemDoRemetente({ url: 'não é url' })).toBeNull();
    expect(origemDoRemetente({})).toBeNull();
  });
});

describe('hostDaOrigem', () => {
  it('mostra o host com a porta', () => {
    expect(hostDaOrigem('https://demot.confidata.app')).toBe('demot.confidata.app');
    expect(hostDaOrigem('http://localhost:3000')).toBe('localhost:3000');
  });
});
