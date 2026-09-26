import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { describe, expect, it } from 'vitest';

import { manifesto } from '../manifest.base';

const extensaoDev = JSON.parse(readFileSync(resolve(__dirname, '..', '..', 'protocolo', 'extensao-dev.json'), 'utf8')) as { chave: string; idChrome: string };

describe('manifesto', () => {
  it('pede só nativeMessaging e storage, sem host, sem rede e sem código remoto', () => {
    for (const alvo of ['chrome', 'firefox'] as const) {
      for (const dev of [false, true]) {
        const m = manifesto({ alvo, versao: '1.0.0', dev, chaveDev: extensaoDev.chave });
        expect(m.permissions, `${alvo} ${dev}`).toEqual(['nativeMessaging', 'storage']);
        expect(m).not.toHaveProperty('host_permissions');
        expect(m).not.toHaveProperty('optional_permissions');
        expect(m).not.toHaveProperty('optional_host_permissions');
        expect(m).not.toHaveProperty('content_security_policy');
        expect(m).not.toHaveProperty('externally_connectable');
        expect(m).not.toHaveProperty('web_accessible_resources');
        expect(m.manifest_version).toBe(3);
      }
    }
  });

  it('o script de conteúdo entra só nos emissores, no topo, desde o início; o de dev acrescenta localhost', () => {
    const loja = manifesto({ alvo: 'chrome', versao: '1.0.0', dev: false });
    expect(loja.content_scripts).toEqual([{ matches: ['https://*.confidata.app/*', 'https://ushield.app/*'], js: ['conteudo.js'], run_at: 'document_start', all_frames: false }]);
    const dev = manifesto({ alvo: 'firefox', versao: '1.0.0', dev: true });
    expect((dev.content_scripts as { matches: string[] }[])[0]?.matches).toEqual(['https://*.confidata.app/*', 'https://ushield.app/*', 'http://localhost/*', 'http://*.localhost/*']);
  });

  it('o Chrome da loja sai SEM key; o de dev leva a chave pública que fixa o ID do programa de dev', () => {
    expect(manifesto({ alvo: 'chrome', versao: '1.0.0', dev: false, chaveDev: extensaoDev.chave })).not.toHaveProperty('key');
    const dev = manifesto({ alvo: 'chrome', versao: '1.0.0', dev: true, chaveDev: extensaoDev.chave });
    expect(dev.key).toBe(extensaoDev.chave);
    expect(dev.background).toEqual({ service_worker: 'fundo.js' });
    expect(() => manifesto({ alvo: 'chrome', versao: '1.0.0', dev: true })).toThrow(/chave pública/);
  });

  it('o Firefox leva o ID que o manifesto do programa aceita, e nunca key', () => {
    const m = manifesto({ alvo: 'firefox', versao: '1.0.0', dev: true, chaveDev: extensaoDev.chave });
    expect(m).not.toHaveProperty('key');
    expect(m.background).toEqual({ scripts: ['fundo.js'] });
    // O certificado entregue à página tem nome e CPF: declarar "none" seria falso.
    expect(m.browser_specific_settings).toMatchObject({
      gecko: { id: 'assinador@confidata.com.br', data_collection_permissions: { required: ['personallyIdentifyingInfo'], optional: ['technicalAndInteraction'] } },
    });
  });

  it('a versão é X.Y.Z (a forma que a biblioteca compara com a versão mínima)', () => {
    expect(() => manifesto({ alvo: 'chrome', versao: '1.0.0-dev', dev: false })).toThrow(/X\.Y\.Z/);
    const pacote = JSON.parse(readFileSync(resolve(__dirname, '..', 'package.json'), 'utf8')) as { version: string };
    expect(pacote.version).toMatch(/^\d+\.\d+\.\d+$/);
  });

  it('o ID de desenvolvimento é o que a chave pública deriva (o que o programa de dev aceita)', async () => {
    const { createHash } = await import('node:crypto');
    const hex = createHash('sha256').update(Buffer.from(extensaoDev.chave, 'base64')).digest('hex').slice(0, 32);
    const id = [...hex].map((c) => String.fromCharCode(97 + parseInt(c, 16))).join('');
    expect(id).toBe(extensaoDev.idChrome);
  });
});
