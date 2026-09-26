/**
 * Textos das páginas da extensão, pelo `i18n` do navegador (`_locales/pt_BR`, o padrão, e
 * `_locales/es`). Nenhum texto visível mora no código: a página marca o elemento com
 * `data-i18n="<chave>"` (conteúdo), `data-i18n-title` (atributo `title`) ou `data-i18n-aria-label`,
 * e `aplicarTextos` preenche. A chave que falta volta como a própria chave, para aparecer no teste
 * de paridade e na tela, e nunca como texto vazio.
 */

export type Traduzir = (chave: string, substituicoes?: string | string[]) => string;

/** O tradutor do navegador. */
export function tradutorDoNavegador(api: typeof chrome): Traduzir {
  return (chave, substituicoes) => api.i18n.getMessage(chave, substituicoes) || chave;
}

/** O idioma da interface do navegador, para o `lang` do documento e a data (`pt-BR`, `es-419`). */
export function idiomaDoNavegador(api: typeof chrome): string {
  return api.i18n.getUILanguage();
}

/** Preenche os elementos marcados sob `raiz`. `textContent`, nunca `innerHTML`. */
export function aplicarTextos(raiz: ParentNode, t: Traduzir): void {
  for (const el of raiz.querySelectorAll<HTMLElement>('[data-i18n]')) {
    const chave = el.dataset.i18n;
    if (chave) el.textContent = t(chave);
  }
  for (const el of raiz.querySelectorAll<HTMLElement>('[data-i18n-title]')) {
    const chave = el.dataset.i18nTitle;
    if (chave) el.title = t(chave);
  }
  for (const el of raiz.querySelectorAll<HTMLElement>('[data-i18n-aria-label]')) {
    const chave = el.dataset.i18nAriaLabel;
    if (chave) el.setAttribute('aria-label', t(chave));
  }
}

/**
 * Uma data RFC 3339 como a pessoa lê (`26/09/2027`), no idioma do navegador. A validade do
 * certificado sai em UTC (`emUtc`, o padrão: o dia que o emissor gravou); um ato da própria pessoa
 * (quando permitiu um endereço) sai no fuso do computador. Texto sem forma volta como veio.
 */
export function dataParaLer(rfc3339: string, idioma: string, emUtc = true): string {
  const d = new Date(rfc3339);
  if (Number.isNaN(d.getTime())) return rfc3339;
  try {
    return new Intl.DateTimeFormat(idioma, { day: '2-digit', month: '2-digit', year: 'numeric', ...(emUtc ? { timeZone: 'UTC' } : {}) }).format(d);
  } catch {
    return d.toISOString().slice(0, 10);
  }
}
