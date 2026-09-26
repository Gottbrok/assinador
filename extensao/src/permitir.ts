/**
 * `permitir.html`: a pessoa autoriza um endereço a usar o Assinador neste computador (§3.3 do
 * plano). O endereço vem do fundo, que o leu do NAVEGADOR (a origem de quem pediu), nunca da
 * página. Esc e fechar a janela são "Não permitir".
 */

import { navegador } from './api';
import { aplicarTextos, idiomaDoNavegador, tradutorDoNavegador } from './i18n';
import { destravarDepoisDoAtraso, dormirDeVerdade, enviarDecisao, lerFluxo, pedirDados, type Enviar } from './janela';

/** O host que a janela mostra, ou `null` se os dados não têm forma. */
export function lerHost(valor: unknown): string | null {
  if (typeof valor !== 'object' || valor === null) return null;
  const host = (valor as { host?: unknown }).host;
  return typeof host === 'string' && host.length > 0 && host.length <= 256 ? host : null;
}

function el<T extends HTMLElement>(id: string): T {
  const e = document.getElementById(id);
  if (!e) throw new Error(`#${id} ausente em permitir.html`);
  return e as T;
}

async function iniciar(api: typeof chrome): Promise<void> {
  const t = tradutorDoNavegador(api);
  document.documentElement.lang = idiomaDoNavegador(api);
  aplicarTextos(document, t);

  const estado = el<HTMLParagraphElement>('estado');
  const enviar: Enviar = (m) => api.runtime.sendMessage(m);
  const fluxo = lerFluxo(location.search);
  const host = fluxo ? lerHost(await pedirDados(enviar, fluxo, dormirDeVerdade)) : null;
  if (!fluxo || !host) {
    estado.textContent = t('janelaExpirou');
    return;
  }
  el('host').textContent = host;
  estado.hidden = true;
  el('conteudo').hidden = false;

  const permitir = el<HTMLButtonElement>('permitir');
  const negar = el<HTMLButtonElement>('negar');
  destravarDepoisDoAtraso(document, [permitir, negar]);
  // O foco nasce em "Não permitir": um Enter que chegue junto com a janela nega, nunca autoriza.
  negar.focus();

  let decidido = false;
  const decidir = async (valor: boolean) => {
    if (decidido) return;
    decidido = true;
    permitir.disabled = true;
    negar.disabled = true;
    if (!(await enviarDecisao(enviar, fluxo, { permitir: valor }))) {
      el('conteudo').hidden = true;
      estado.textContent = t('janelaExpirou');
      estado.hidden = false;
    }
  };
  permitir.addEventListener('click', () => {
    if (!permitir.disabled) void decidir(true);
  });
  negar.addEventListener('click', () => {
    if (!negar.disabled) void decidir(false);
  });
  document.addEventListener('keydown', (evento) => {
    if (evento.key === 'Escape') void decidir(false);
  });
}

const api = navegador();
if (api && typeof document !== 'undefined') void iniciar(api);
