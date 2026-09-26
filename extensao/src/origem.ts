/**
 * Quem pode falar com a extensão.
 *
 * Uma página só alcança a extensão se estiver nos padrões de algum emissor de bilhete (§3.4 do
 * plano): o Confidata em `https://<organização>.confidata.app` e o ushield em `https://ushield.app`.
 * O build de desenvolvimento acrescenta `localhost`. O `content_scripts.matches` do manifesto já
 * restringe onde o script entra; conferir de novo aqui, no fundo, é o que vale, porque o manifesto
 * casa por padrão largo (`*.confidata.app`) e a conferência é pela expressão exata da fixture.
 */

import { PADROES_DE_ORIGEM, PADROES_DE_ORIGEM_DEV } from './protocolo';

const REGEX_DOS_EMISSORES = Object.values(PADROES_DE_ORIGEM)
  .flat()
  .map((p) => new RegExp(p));
const REGEX_DEV = PADROES_DE_ORIGEM_DEV.map((p) => new RegExp(p));

/** A origem está nos padrões de algum emissor (e, no build `dev`, também em `localhost`)? */
export function origemAceita(origem: string, dev: boolean): boolean {
  if (typeof origem !== 'string' || origem.length === 0 || origem.length > 256) return false;
  if (REGEX_DOS_EMISSORES.some((r) => r.test(origem))) return true;
  return dev && REGEX_DEV.some((r) => r.test(origem));
}

/** O remetente de uma mensagem, no que a conferência olha. */
export interface Remetente {
  id?: string | undefined;
  origin?: string | undefined;
  url?: string | undefined;
  frameId?: number | undefined;
  tab?: { id?: number | undefined; windowId?: number | undefined } | undefined;
}

/**
 * A origem de quem mandou a mensagem, dita pelo NAVEGADOR: `sender.origin` no Chrome e no Edge; no
 * Firefox, que não o preenche, a origem de `sender.url`. Nunca a origem que a própria mensagem
 * declara. `null` quando nenhum dos dois serve.
 */
export function origemDoRemetente(remetente: Remetente): string | null {
  if (typeof remetente.origin === 'string' && remetente.origin !== 'null' && remetente.origin.length > 0) return remetente.origin;
  if (typeof remetente.url === 'string') {
    try {
      const o = new URL(remetente.url).origin;
      return o === 'null' ? null : o;
    } catch {
      return null;
    }
  }
  return null;
}

/** O host que a pessoa lê nas janelas (`demot.confidata.app`, `localhost:3000`). */
export function hostDaOrigem(origem: string): string {
  try {
    return new URL(origem).host;
  } catch {
    return origem;
  }
}
