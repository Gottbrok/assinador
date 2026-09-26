/**
 * `confirmar.html`: a pessoa confere o que vai assinar e decide (§3.6 do plano).
 *
 * Tudo o que a janela mostra veio do fundo, que o leu do PROGRAMA (o bilhete conferido: organização,
 * documento, finalidade; o certificado achado no dispositivo) e do NAVEGADOR (o endereço de quem
 * pediu). A página que pediu a assinatura não alcança esta janela. O PIN só aparece quando o
 * dispositivo o exige; sem ele (leitora com teclado, diálogo do fabricante) a janela avisa onde
 * digitar. Enter no campo do PIN assina, Esc cancela, e fechar a janela é cancelar. Os botões
 * obedecem à trava de `janela.ts` (visível, com foco, 600 ms).
 */

import { navegador } from './api';
import { aplicarTextos, dataParaLer, idiomaDoNavegador, tradutorDoNavegador, type Traduzir } from './i18n';
import { ambienteDoDocumento, dormirDeVerdade, enviarDecisao, ignorarTeclaRepetida, lerFluxo, pedirDados, travarAteVer, type Enviar } from './janela';

/** O que a janela recebe do fundo (`DadosDaConfirmacao` em `fundo.ts`), conferido de novo aqui. */
export interface Confirmacao {
  host: string;
  organizacao: string;
  documento: string;
  finalidade: string;
  certificado: { titular: string; emissor: string; validoAte: string };
  exigePin: boolean;
  estadoDoPin: string | null;
}

function texto(v: unknown): v is string {
  return typeof v === 'string';
}

export function lerConfirmacao(valor: unknown): Confirmacao | null {
  if (typeof valor !== 'object' || valor === null) return null;
  const v = valor as Record<string, unknown>;
  const c = v.certificado as Record<string, unknown> | null | undefined;
  if (!texto(v.host) || !texto(v.organizacao) || !texto(v.documento) || !texto(v.finalidade)) return null;
  if (typeof c !== 'object' || c === null || !texto(c.titular) || !texto(c.emissor) || !texto(c.validoAte)) return null;
  if (typeof v.exigePin !== 'boolean' || (v.estadoDoPin !== null && !texto(v.estadoDoPin))) return null;
  return {
    host: v.host,
    organizacao: v.organizacao,
    documento: v.documento,
    finalidade: v.finalidade,
    certificado: { titular: c.titular, emissor: c.emissor, validoAte: c.validoAte },
    exigePin: v.exigePin,
    estadoDoPin: v.estadoDoPin,
  };
}

/** A identidade (`verificacao`, do ushield) confirma; o resto assina. */
export function ehVerificacao(finalidade: string): boolean {
  return finalidade === 'verificacao';
}

/** A frase da finalidade. Valor fora do protocolo aparece como veio (o programa já tirou os caracteres de controle). */
export function textoDaFinalidade(finalidade: string, t: Traduzir): string {
  if (finalidade === 'assinatura') return t('finalidadeAssinatura');
  if (finalidade === 'verificacao') return t('finalidadeVerificacao');
  return finalidade;
}

/** A chave do aviso de tentativas de PIN, ou `null` quando não há o que avisar. */
export function chaveDoAvisoDoPin(estado: string | null): string | null {
  if (estado === 'poucas-tentativas') return 'avisoPoucasTentativas';
  if (estado === 'ultima-tentativa') return 'avisoUltimaTentativa';
  return null;
}

function el<T extends HTMLElement>(id: string): T {
  const e = document.getElementById(id);
  if (!e) throw new Error(`#${id} ausente em confirmar.html`);
  return e as T;
}

async function iniciar(api: typeof chrome): Promise<void> {
  const t = tradutorDoNavegador(api);
  const idioma = idiomaDoNavegador(api);
  document.documentElement.lang = idioma;
  aplicarTextos(document, t);

  const estado = el<HTMLParagraphElement>('estado');
  const enviar: Enviar = (m) => api.runtime.sendMessage(m);
  const fluxo = lerFluxo(location.search);
  const dados = fluxo ? lerConfirmacao(await pedirDados(enviar, fluxo, dormirDeVerdade)) : null;
  if (!fluxo || !dados) {
    estado.textContent = t('janelaExpirou');
    return;
  }

  const verificacao = ehVerificacao(dados.finalidade);
  const titulo = t(verificacao ? 'confirmarIdentidadeTitulo' : 'confirmarTitulo');
  document.title = titulo;
  el('titulo').textContent = titulo;
  el('host').textContent = dados.host;
  el('organizacao').textContent = dados.organizacao;
  el('documento').textContent = dados.documento;
  el('finalidade').textContent = textoDaFinalidade(dados.finalidade, t);
  el('titular').textContent = dados.certificado.titular;
  el('emissor').textContent = t('emitidoPor', dados.certificado.emissor);
  el('validade').textContent = t('validoAte', dataParaLer(dados.certificado.validoAte, idioma));

  const chaveDoAviso = chaveDoAvisoDoPin(dados.estadoDoPin);
  if (chaveDoAviso) {
    const aviso = el('aviso-pin');
    aviso.textContent = t(chaveDoAviso);
    aviso.hidden = false;
  }

  const pin = el<HTMLInputElement>('pin');
  const erroDoPin = el<HTMLParagraphElement>('erro-pin');
  const assinar = el<HTMLButtonElement>('assinar');
  const cancelar = el<HTMLButtonElement>('cancelar');
  assinar.textContent = t(verificacao ? 'botaoConfirmar' : 'botaoAssinar');
  el('campo-pin').hidden = !dados.exigePin;
  el('pin-no-leitor').hidden = dados.exigePin;
  estado.hidden = true;
  el('conteudo').hidden = false;
  ignorarTeclaRepetida(document);
  // O foco nunca nasce em "Assinar": com o PIN na janela, vai ao campo; sem ele (leitora com
  // teclado), vai a "Cancelar", e assinar exige um gesto da pessoa depois da trava.
  const trava = travarAteVer(
    ambienteDoDocumento(document, window),
    (travado) => {
      assinar.disabled = travado;
      cancelar.disabled = travado;
    },
    () => (dados.exigePin ? pin : cancelar).focus(),
  );
  if (dados.exigePin) pin.focus();

  let decidido = false;
  const decidir = async (valor: { assinar: boolean; pin?: string }) => {
    if (decidido) return;
    decidido = true;
    trava.encerrar();
    assinar.disabled = true;
    cancelar.disabled = true;
    pin.disabled = true;
    const aceito = await enviarDecisao(enviar, fluxo, valor);
    pin.value = '';
    if (!aceito) {
      el('conteudo').hidden = true;
      estado.textContent = t('janelaExpirou');
      estado.hidden = false;
    }
  };

  el<HTMLFormElement>('formulario').addEventListener('submit', (evento) => {
    evento.preventDefault();
    if (assinar.disabled) return;
    if (dados.exigePin && pin.value.length === 0) {
      erroDoPin.textContent = t('digitePin');
      erroDoPin.hidden = false;
      pin.focus();
      return;
    }
    void decidir(dados.exigePin ? { assinar: true, pin: pin.value } : { assinar: true });
  });
  cancelar.addEventListener('click', () => {
    if (!cancelar.disabled) void decidir({ assinar: false });
  });
  // Esc cancela mesmo durante a trava: cancelar nunca é o gesto perigoso.
  document.addEventListener('keydown', (evento) => {
    if (evento.key === 'Escape') void decidir({ assinar: false });
  });
}

const api = navegador();
if (api && typeof document !== 'undefined') void iniciar(api);
