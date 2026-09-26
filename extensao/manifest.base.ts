/**
 * O manifesto da extensão, uma fonte para os dois alvos (§3.6 do plano):
 *
 *  - `chrome` (Chrome e Edge): fundo em `background.service_worker`; no build de desenvolvimento, a
 *    chave PÚBLICA de `protocolo/extensao-dev.json` em `key`, que fixa o ID que o programa de
 *    desenvolvimento aceita. O pacote da loja sai SEM `key`: a Chrome Web Store recusa o campo (o ID
 *    vem da loja, no rascunho do item).
 *  - `firefox`: fundo em `background.scripts` e o ID fixo `assinador@confidata.com.br`, o mesmo do
 *    `allowed_extensions` do manifesto do programa.
 *
 * Permissões: `nativeMessaging` (falar com o programa) e `storage` (a lista de endereços permitidos,
 * em `storage.local`, nunca sincronizada). Nenhuma permissão de host e nenhum acesso de rede: o script
 * de conteúdo entra só nos endereços dos emissores de bilhete, e o de desenvolvimento acrescenta
 * `localhost`. O Plano escreveu "`nativeMessaging` e nada mais"; a lista de permissões por endereço,
 * que o mesmo plano exige, precisa de `storage` (correção registrada no plano, F3).
 *
 * Este arquivo roda no Node (o `build.mjs` o importa, com a remoção de tipos do Node 22.18+) e no
 * Vitest: só sintaxe de tipos apagável, e nenhum import.
 */

export type Alvo = 'chrome' | 'firefox';

export interface OpcoesDoManifesto {
  alvo: Alvo;
  versao: string;
  /** Build de desenvolvimento: acrescenta `localhost` e, no Chrome, a chave pública de dev. */
  dev: boolean;
  /** Chave pública de desenvolvimento (base64 do SPKI), obrigatória no Chrome com `dev`. */
  chaveDev?: string;
}

/** O ID da extensão no Firefox (`origem.ExtensaoFirefox` no programa). */
export const ID_NO_FIREFOX = 'assinador@confidata.com.br';

/** Onde o script de conteúdo entra: os padrões dos emissores (a fixture tem as expressões exatas). */
export const ENDERECOS_DOS_EMISSORES = ['https://*.confidata.app/*', 'https://ushield.app/*'] as const;
export const ENDERECOS_DE_DESENVOLVIMENTO = ['http://localhost/*', 'http://*.localhost/*'] as const;

/** Chrome 116: a porta de native messaging aberta mantém o service worker vivo. */
export const CHROME_MINIMO = '116';
/** Firefox 140 (ESR): `data_collection_permissions` no manifesto. */
export const FIREFOX_MINIMO = '140.0';

export function manifesto(o: OpcoesDoManifesto): Record<string, unknown> {
  if (!/^\d{1,5}\.\d{1,5}\.\d{1,5}$/.test(o.versao)) throw new Error(`versão fora da forma X.Y.Z: ${o.versao}`);
  if (o.alvo === 'chrome' && o.dev && !o.chaveDev) throw new Error('o build dev do Chrome precisa da chave pública de desenvolvimento');
  const matches = o.dev ? [...ENDERECOS_DOS_EMISSORES, ...ENDERECOS_DE_DESENVOLVIMENTO] : [...ENDERECOS_DOS_EMISSORES];
  const base: Record<string, unknown> = {
    manifest_version: 3,
    name: '__MSG_nomeDaExtensao__',
    description: '__MSG_descricaoDaExtensao__',
    version: o.versao,
    default_locale: 'pt_BR',
    icons: { '16': 'icones/16.png', '32': 'icones/32.png', '48': 'icones/48.png', '128': 'icones/128.png' },
    permissions: ['nativeMessaging', 'storage'],
    content_scripts: [{ matches, js: ['conteudo.js'], run_at: 'document_start', all_frames: false }],
    options_ui: { page: 'opcoes.html', open_in_tab: true },
  };
  if (o.alvo === 'chrome') {
    return {
      ...base,
      ...(o.dev && o.chaveDev ? { key: o.chaveDev } : {}),
      minimum_chrome_version: CHROME_MINIMO,
      background: { service_worker: 'fundo.js' },
    };
  }
  return {
    ...base,
    background: { scripts: ['fundo.js'] },
    browser_specific_settings: {
      gecko: {
        id: ID_NO_FIREFOX,
        strict_min_version: FIREFOX_MINIMO,
        // A extensão não coleta nem transmite dado nenhum (§3.6: sem acesso de rede).
        data_collection_permissions: { required: ['none'] },
      },
    },
  };
}
