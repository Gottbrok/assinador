/**
 * O fundo da extensão (service worker no Chrome e no Edge, script de fundo no Firefox).
 *
 * Recebe o pedido que o script de conteúdo repassou (uma PORTA por pedido, que cai quando a página
 * sai) e o atende, em quatro portões, nesta ordem:
 *
 *  1. o REMETENTE é o script de conteúdo desta extensão, no quadro de topo de uma aba
 *     (`sender.id`, `sender.frameId === 0`, aba presente);
 *  2. a ORIGEM que o navegador diz do remetente é a que a mensagem declara, e está nos padrões de
 *     algum emissor (e `localhost` no build de desenvolvimento);
 *  3. a PERMISSÃO da pessoa para aquele endereço, para tudo que não é `ola` (a janela
 *     `permitir.html` pergunta na primeira vez);
 *  4. a FORMA dos dados, e só então o programa.
 *
 * `assinar` é um fluxo: `conferir` no programa (que confere o bilhete e acha o certificado), a
 * janela `confirmar.html` com o que o BILHETE diz (o documento, a organização) e o endereço que o
 * NAVEGADOR diz, a decisão da pessoa com o PIN quando o token exige, e `assinar` na mesma porta.
 * Um fluxo de assinatura por vez: o segundo recebe `ocupado`. A página que sai ou o programa que cai
 * encerram o fluxo e fecham a janela, em vez de deixá-la esperando um clique que ninguém recebe.
 *
 * Cada operação da página tem um ORÇAMENTO (`PRAZOS_DO_FLUXO_MS`), abaixo do prazo da página, e
 * cada passo usa o menor entre o teto dele e o que resta (`prazoDoPasso`). O `ola` é um só em voo e
 * vale por alguns segundos; `listar` e `diagnostico` passam pela fila do dispositivo; e um endereço
 * que recusa janela atrás de janela fica embargado por alguns minutos.
 */

import { navegador } from './api';
import { criarEmbargo, type Embargo } from './embargo';
import { criarFila, type Fila } from './fila';
import { controleDoNavegador, criarJanelas, type Janelas } from './janelas';
import { abrirPrograma, portaDoNavegador, relogioReal, type PortaNativa, type Programa, type Relogio } from './nativo';
import { hostDaOrigem, origemAceita, origemDoRemetente, type Remetente } from './origem';
import { armazenamentoLocal, permitido, permitir, type Armazenamento } from './permissoes';
import {
  EMBARGO_DA_CONFIRMACAO,
  EMBARGO_DA_PERMISSAO,
  falha,
  FILA_DO_DISPOSITIVO,
  FORMA_DO_BILHETE,
  FORMA_HEX64,
  OPERACOES_DA_PAGINA,
  ORIGEM_DA_EXTENSAO,
  PORTA_DO_PEDIDO,
  PRAZO_DA_DECISAO_MS,
  PRAZO_DA_PERMISSAO_MS,
  PRAZOS_DO_FLUXO_MS,
  PRAZOS_DO_PROGRAMA_MS,
  RESERVA_DEPOIS_DA_CONFIRMACAO_MS,
  RESERVA_DEPOIS_DA_PERMISSAO_MS,
  TAMANHO_MAXIMO_DO_BILHETE,
  VALIDADE_DO_OLA_MS,
  type OperacaoDaPagina,
  type Resultado,
} from './protocolo';

export interface AmbienteDoFundo {
  idDaExtensao: string;
  /**
   * A raiz das páginas desta extensão, com a barra final (`chrome-extension://<id>/`,
   * `moz-extension://<uuid>/`), como `runtime.getURL('')` a dá. Comparar por prefixo da URL, e não
   * por `new URL(...).origin`: esquema de extensão não é esquema especial do WHATWG, e a `origin`
   * dele pode sair `"null"` conforme o navegador.
   */
  urlDaExtensao: string;
  versao: string;
  dev: boolean;
  navegador: string;
  conectar: () => PortaNativa;
  armazenamento: Armazenamento;
  janelas: Janelas;
  novoId: () => string;
  relogio: Relogio;
  /**
   * O diagnóstico pode ir a uma PÁGINA? No Firefox, entregar dado técnico ao site é transmissão, e
   * ela é o tipo opcional `technicalAndInteraction`, que a pessoa consente (na instalação ou nas
   * opções). No Chrome e no Edge, sempre. As opções da própria extensão não passam por aqui: mostrar
   * o relatório a quem o pediu não é transmissão.
   */
  podeEntregarDiagnostico: () => Promise<boolean>;
}

/** O que a janela de confirmação mostra: tudo vem do programa (que leu o bilhete) e do navegador. */
export interface DadosDaConfirmacao {
  host: string;
  organizacao: string;
  documento: string;
  finalidade: string;
  certificado: { titular: string; emissor: string; validoAte: string };
  exigePin: boolean;
  estadoDoPin: string | null;
}

export interface DecisaoDaConfirmacao {
  assinar: boolean;
  pin?: string;
}

/**
 * As linhas que a extensão acrescenta ao diagnóstico. O diagnóstico é artefato do SUPORTE e sai em
 * português, como as frases do programa (decisão da F2b): uma linha em espanhol no meio de um
 * relatório em português só atrapalharia quem o lê.
 */
export const ROTULOS_DO_DIAGNOSTICO = Object.freeze({ extensao: 'Extensão: versão', navegador: 'Navegador:' });

function ehObjeto(valor: unknown): valor is Record<string, unknown> {
  return typeof valor === 'object' && valor !== null && !Array.isArray(valor);
}

function ehOperacao(op: unknown): op is OperacaoDaPagina {
  return typeof op === 'string' && (OPERACOES_DA_PAGINA as readonly string[]).includes(op);
}

/** Quanto um passo pode durar: o teto dele ou o que resta do orçamento menos a reserva do passo seguinte. */
export function prazoDoPasso(teto: number, resta: number, reserva = 0): number {
  return Math.min(teto, resta - reserva);
}

/** Os dados de `assinar` com forma estrita, ou `null`. */
export function lerDadosDoAssinar(dados: unknown): { ref: string; digest: string; bilhete: string } | null {
  if (!ehObjeto(dados)) return null;
  const chaves = Object.keys(dados);
  if (chaves.length !== 3 || !chaves.every((k) => k === 'ref' || k === 'digest' || k === 'bilhete')) return null;
  const { ref, digest, bilhete } = dados;
  if (typeof ref !== 'string' || !FORMA_HEX64.test(ref)) return null;
  if (typeof digest !== 'string' || !FORMA_HEX64.test(digest)) return null;
  if (typeof bilhete !== 'string' || bilhete.length > TAMANHO_MAXIMO_DO_BILHETE || !FORMA_DO_BILHETE.test(bilhete)) return null;
  return { ref, digest, bilhete };
}

/** O titular que a janela mostra: o CN antes do `:` (o CN ICP-Brasil é `NOME:CPF`, e o programa já mascarou os dígitos). */
export function titularDoAssunto(assunto: string): string {
  const i = assunto.indexOf(':');
  return i > 0 ? assunto.slice(0, i) : assunto;
}

/** A resposta do `conferir`, lida para a janela, ou `null` se não tiver forma. */
export function lerConferir(dados: Record<string, unknown>, host: string): DadosDaConfirmacao | null {
  const { organizacao, documento, finalidade, certificado } = dados;
  if (typeof organizacao !== 'string' || typeof documento !== 'string' || typeof finalidade !== 'string' || !ehObjeto(certificado)) return null;
  const { assunto, emissor, validoAte, exigePin, estadoDoPin } = certificado;
  if (typeof assunto !== 'string' || typeof emissor !== 'string' || typeof validoAte !== 'string' || typeof exigePin !== 'boolean') return null;
  if (estadoDoPin !== undefined && typeof estadoDoPin !== 'string') return null;
  return {
    host,
    organizacao,
    documento,
    finalidade,
    certificado: { titular: titularDoAssunto(assunto), emissor, validoAte },
    exigePin,
    estadoDoPin: typeof estadoDoPin === 'string' ? estadoDoPin : null,
  };
}

/** A decisão da janela, com forma estrita, ou `null`. O PIN segue as regras do programa (até 64 bytes, sem controle). */
export function lerDecisao(valor: unknown): DecisaoDaConfirmacao | null {
  if (!ehObjeto(valor) || typeof valor.assinar !== 'boolean') return null;
  if (!valor.assinar) return { assinar: false };
  if (valor.pin === undefined) return { assinar: true };
  if (typeof valor.pin !== 'string' || valor.pin.length === 0 || new TextEncoder().encode(valor.pin).length > 64 || /[\u0000-\u001f\u007f]/.test(valor.pin)) return null;
  return { assinar: true, pin: valor.pin };
}

/**
 * A mensagem veio de uma página DESTA extensão (janela ou opções)? O `sender.id` é o desta extensão
 * também para o script de conteúdo, então o que decide é a URL do remetente: página da extensão
 * mora sob `urlDaExtensao`; o script de conteúdo tem a URL da página web.
 */
export function daPropriaExtensao(remetente: Remetente, amb: Pick<AmbienteDoFundo, 'idDaExtensao' | 'urlDaExtensao'>): boolean {
  if (remetente.id !== amb.idDaExtensao || !amb.urlDaExtensao.endsWith('/')) return false;
  if (typeof remetente.url === 'string') return remetente.url.startsWith(amb.urlDaExtensao);
  return typeof remetente.origin === 'string' && `${remetente.origin}/` === amb.urlDaExtensao;
}

export interface Fundo {
  /**
   * Um pedido vindo do script de conteúdo. `saiu` resolve quando a página sai (a porta do pedido
   * caiu): o fluxo se encerra e a janela aberta fecha. Nunca rejeita.
   */
  atenderPagina(mensagem: unknown, remetente: Remetente, saiu?: Promise<void>): Promise<Resultado>;
  /** Uma mensagem vinda de uma página da própria extensão (janela ou opções). */
  atenderExtensao(mensagem: unknown, remetente: Remetente): Promise<unknown>;
}

const EMBARGADO = 'embargo: janelas recusadas seguidas deste endereço; tente de novo em alguns minutos';

/**
 * `tarefa` ou, se a página sair antes, `seSair`. Sem `saiu` (as opções da extensão, os testes), só a
 * tarefa: correr contra uma promessa que nunca resolve deixaria uma reação pendurada por pedido.
 */
function ateAPaginaSair<T>(tarefa: Promise<T>, saiu: Promise<void> | undefined, seSair: T): Promise<T> {
  return saiu ? Promise.race([tarefa, saiu.then(() => seSair)]) : tarefa;
}

export function criarFundo(amb: AmbienteDoFundo): Fundo {
  let assinando = false;
  const permissoesPendentes = new Map<string, Promise<Resultado | null>>();
  const fila: Fila = criarFila(amb.relogio, FILA_DO_DISPOSITIVO);
  const embargoDaPermissao: Embargo = criarEmbargo(EMBARGO_DA_PERMISSAO, amb.relogio);
  const embargoDaConfirmacao: Embargo = criarEmbargo(EMBARGO_DA_CONFIRMACAO, amb.relogio);
  let olaEmVoo: Promise<Resultado> | null = null;
  let olaGuardado: { quando: number; resultado: Resultado } | null = null;

  const programa = (): Programa => abrirPrograma(amb.conectar, amb.novoId, amb.relogio);

  /** O que resta do orçamento (`limite` é um instante de `relogio.agora()`); sem limite, sem teto. */
  const resta = (limite: number | null): number => (limite === null ? Number.POSITIVE_INFINITY : limite - amb.relogio.agora());

  async function umPedido(op: 'ola' | 'listar' | 'diagnostico', origem: string, limite: number | null, saiu?: Promise<void>): Promise<Resultado> {
    const p = programa();
    // A página que sai fecha a porta: o pedido volta `cancelado` e o processo do programa sai.
    void saiu?.then(() => p.fechar());
    try {
      return await p.pedir(op, origem, undefined, resta(limite));
    } finally {
      p.fechar();
    }
  }

  /** Um pedido que carrega os programas do cartão: pela fila do dispositivo, dentro do orçamento. */
  function pedidoDoDispositivo(op: 'listar' | 'diagnostico', origem: string, limite: number | null, saiu?: Promise<void>): Promise<Resultado> {
    const espera = limite === null ? PRAZOS_DO_PROGRAMA_MS[op] : resta(limite);
    if (espera <= 0) return Promise.resolve(falha('tempo-esgotado', `sem tempo para ${op}`));
    return fila.executar(espera, () => umPedido(op, origem, limite, saiu));
  }

  async function perguntarOla(limite: number | null): Promise<Resultado> {
    const r = await umPedido('ola', ORIGEM_DA_EXTENSAO, limite);
    const extensao = { versao: amb.versao };
    if (r.ok) {
      const { versao, protocolo, plataforma } = r.dados;
      if (typeof versao === 'string' && typeof plataforma === 'string' && Number.isInteger(protocolo)) {
        return { ok: true, dados: { extensao, nativo: { versao, protocolo, plataforma } } };
      }
      return { ok: true, dados: { extensao, nativo: null, motivoNativo: 'falhou' } };
    }
    return { ok: true, dados: { extensao, nativo: null, motivoNativo: r.erro.codigo === 'nativo-ausente' ? 'ausente' : 'falhou' } };
  }

  /**
   * O `ola`. O programa responde o mesmo a qualquer origem (só versões), então vale um em voo para
   * todos e a resposta serve por `VALIDADE_DO_OLA_MS`: uma página em laço não abre um processo do
   * programa por pedido.
   */
  function ola(limite: number | null): Promise<Resultado> {
    if (olaGuardado && amb.relogio.agora() - olaGuardado.quando < VALIDADE_DO_OLA_MS) return Promise.resolve(olaGuardado.resultado);
    if (!olaEmVoo) {
      olaEmVoo = perguntarOla(limite).then(
        (resultado) => {
          olaGuardado = { quando: amb.relogio.agora(), resultado };
          olaEmVoo = null;
          return resultado;
        },
        (erro: unknown) => {
          olaEmVoo = null;
          return falha('interno', erro instanceof Error ? erro.message : String(erro));
        },
      );
    }
    return olaEmVoo;
  }

  /** `null` quando a origem pode seguir; senão, a recusa. */
  async function temPermissao(origem: string, limite: number, saiu?: Promise<void>): Promise<Resultado | null> {
    if (await permitido(amb.armazenamento, origem)) return null;
    // Dois pedidos do mesmo endereço ao mesmo tempo (listar e diagnóstico) abrem UMA janela.
    const pendente = permissoesPendentes.get(origem);
    if (pendente) return pendente;
    if (embargoDaPermissao.emEmbargo(origem)) return falha('permissao-negada', EMBARGADO);
    const prazo = prazoDoPasso(PRAZO_DA_PERMISSAO_MS, resta(limite), RESERVA_DEPOIS_DA_PERMISSAO_MS);
    if (prazo <= 0) return falha('tempo-esgotado', 'sem tempo para pedir a permissão');
    const pergunta = (async (): Promise<Resultado | null> => {
      const d = await amb.janelas.esperar<unknown>('permitir.html', 420, 320, { host: hostDaOrigem(origem) }, prazo, saiu);
      switch (d.tipo) {
        case 'falhou':
          return falha('interno', 'a janela de permissão não abriu');
        case 'interrompida':
          return falha('cancelado', 'a página saiu');
        case 'prazo':
          embargoDaPermissao.registrarRecusa(origem);
          return falha('tempo-esgotado', 'a janela de permissão não foi respondida');
        case 'fechada':
          embargoDaPermissao.registrarRecusa(origem);
          return falha('permissao-negada');
        case 'decisao':
          if (ehObjeto(d.valor) && d.valor.permitir === true) {
            embargoDaPermissao.registrarAceite(origem);
            await permitir(amb.armazenamento, origem);
            return null;
          }
          embargoDaPermissao.registrarRecusa(origem);
          return falha('permissao-negada');
      }
    })();
    permissoesPendentes.set(origem, pergunta);
    try {
      return await pergunta;
    } finally {
      permissoesPendentes.delete(origem);
    }
  }

  async function diagnostico(origem: string, limite: number | null, saiu?: Promise<void>): Promise<Resultado> {
    const r = await pedidoDoDispositivo('diagnostico', origem, limite, saiu);
    if (!r.ok) return r;
    const { relatorio, texto } = r.dados;
    if (!ehObjeto(relatorio) || typeof texto !== 'string') return falha('protocolo', 'diagnóstico sem forma');
    // O programa não sabe de navegador nem da extensão: quem acrescenta é quem sabe.
    return {
      ok: true,
      dados: {
        relatorio: { ...relatorio, extensao: { versao: amb.versao }, navegador: amb.navegador },
        texto: `${ROTULOS_DO_DIAGNOSTICO.extensao} ${amb.versao}\n${ROTULOS_DO_DIAGNOSTICO.navegador} ${amb.navegador}\n${texto}`,
      },
    };
  }

  async function assinar(origem: string, dadosBrutos: unknown, limite: number, saiu?: Promise<void>): Promise<Resultado> {
    const dados = lerDadosDoAssinar(dadosBrutos);
    if (!dados) return falha('protocolo', 'dados de assinar sem forma');
    if (embargoDaConfirmacao.emEmbargo(origem)) return falha('cancelado', EMBARGADO);
    if (assinando) return falha('ocupado', 'outra assinatura em curso');
    assinando = true;
    const p = programa();
    // O que aconteceu enquanto se esperava (atribuído nos ouvintes, lido depois).
    const ocorrido: { paginaSaiu: boolean; quedaDoPrograma: Resultado | null } = { paginaSaiu: false, quedaDoPrograma: null };
    const PAGINA_SAIU = falha('cancelado', 'a página saiu');
    void saiu?.then(() => {
      ocorrido.paginaSaiu = true;
    });
    // A queda do programa com a janela aberta encerra a espera: descobrir só depois do clique
    // faria a pessoa digitar o PIN para nada.
    const programaCaiu = new Promise<void>((resolver) =>
      p.aoCair((recusa) => {
        ocorrido.quedaDoPrograma = recusa;
        resolver();
      }),
    );
    try {
      const conferido = await ateAPaginaSair(p.pedir('conferir', origem, dados, resta(limite)), saiu, PAGINA_SAIU);
      if (!conferido.ok) return conferido;
      const janela = lerConferir(conferido.dados, hostDaOrigem(origem));
      if (!janela) return falha('protocolo', 'conferir sem forma');
      if (janela.estadoDoPin === 'bloqueado') return falha('token-bloqueado', 'o token já está bloqueado');
      const prazo = prazoDoPasso(PRAZO_DA_DECISAO_MS, resta(limite), RESERVA_DEPOIS_DA_CONFIRMACAO_MS);
      if (prazo <= 0) return falha('tempo-esgotado', 'sem tempo para a janela de confirmação');
      const d = await amb.janelas.esperar<unknown>('confirmar.html', 420, 560, janela, prazo, saiu ? Promise.race([saiu, programaCaiu]) : programaCaiu);
      switch (d.tipo) {
        case 'falhou':
          return falha('interno', 'a janela de confirmação não abriu');
        case 'interrompida':
          return !ocorrido.paginaSaiu && ocorrido.quedaDoPrograma ? ocorrido.quedaDoPrograma : PAGINA_SAIU;
        case 'prazo':
          embargoDaConfirmacao.registrarRecusa(origem);
          return falha('tempo-esgotado', 'a janela de confirmação não foi respondida');
        case 'fechada':
          embargoDaConfirmacao.registrarRecusa(origem);
          return falha('cancelado', 'a janela de confirmação foi fechada');
        case 'decisao':
          break;
      }
      const decisao = lerDecisao(d.valor);
      if (!decisao) return falha('protocolo', 'decisão sem forma');
      if (!decisao.assinar) {
        embargoDaConfirmacao.registrarRecusa(origem);
        return falha('cancelado', 'a pessoa cancelou na janela');
      }
      embargoDaConfirmacao.registrarAceite(origem);
      if (janela.exigePin && !decisao.pin) return falha('protocolo', 'o dispositivo exige PIN e ele não veio');
      if (ocorrido.paginaSaiu) return PAGINA_SAIU;
      return await ateAPaginaSair(p.pedir('assinar', origem, janela.exigePin && decisao.pin ? { ...dados, pin: decisao.pin } : dados, resta(limite)), saiu, PAGINA_SAIU);
    } finally {
      p.fechar();
      assinando = false;
    }
  }

  return {
    async atenderPagina(mensagem, remetente, saiu) {
      try {
        if (remetente.id !== amb.idDaExtensao || remetente.frameId !== 0 || typeof remetente.tab?.id !== 'number') return falha('origem-recusada', 'remetente');
        if (!ehObjeto(mensagem) || mensagem.tipo !== 'pedido-da-pagina' || typeof mensagem.origem !== 'string') return falha('protocolo', 'mensagem sem forma');
        const origem = origemDoRemetente(remetente);
        if (origem === null || origem !== mensagem.origem || !origemAceita(origem, amb.dev)) return falha('origem-recusada', 'origem');
        if (!ehOperacao(mensagem.op)) return falha('protocolo', 'operação desconhecida');
        const limite = amb.relogio.agora() + PRAZOS_DO_FLUXO_MS[mensagem.op];
        if (mensagem.op === 'ola') return await ola(limite);
        const recusa = await temPermissao(origem, limite, saiu);
        if (recusa) return recusa;
        switch (mensagem.op) {
          case 'listar':
            return await pedidoDoDispositivo('listar', origem, limite, saiu);
          case 'diagnostico':
            if (!(await amb.podeEntregarDiagnostico())) {
              return falha('permissao-negada', 'o envio do diagnóstico às páginas está desligado nas opções da extensão');
            }
            return await diagnostico(origem, limite, saiu);
          case 'assinar':
            return await assinar(origem, mensagem.dados, limite, saiu);
        }
      } catch (erro) {
        return falha('interno', erro instanceof Error ? erro.message : String(erro));
      }
    },

    async atenderExtensao(mensagem, remetente) {
      // Só as páginas da própria extensão (janelas e opções), nunca um script de conteúdo.
      if (!daPropriaExtensao(remetente, amb)) return null;
      if (!ehObjeto(mensagem)) return null;
      const abaId = remetente.tab?.id;
      switch (mensagem.tipo) {
        case 'dados-da-janela':
          return typeof mensagem.fluxo === 'string' ? amb.janelas.dadosDaJanela(mensagem.fluxo, abaId) : { estado: 'ausente' };
        case 'decisao':
          return typeof mensagem.fluxo === 'string' ? amb.janelas.decidir(mensagem.fluxo, abaId, mensagem.valor) : false;
        case 'versoes': {
          const r = await ola(null);
          return r.ok ? r.dados : null;
        }
        // A página de opções pede o diagnóstico antes de haver endereço autorizado (é o primeiro
        // gesto do suporte); o programa o atende para `ORIGEM_DA_EXTENSAO`, e só ele.
        case 'diagnostico':
          return await diagnostico(ORIGEM_DA_EXTENSAO, null);
        default:
          return null;
      }
    },
  };
}

/** `true` só no build de desenvolvimento (`build.mjs --dev`), que aceita `localhost`. */
declare const __ASSINADOR_DEV__: boolean;
/** O alvo do build (`chrome` ou `firefox`). */
declare const __ASSINADOR_ALVO__: 'chrome' | 'firefox';

/** No Firefox, o consentimento de `technicalAndInteraction`; se a API não responde, fecha. */
function consentimentoDoDiagnostico(api: typeof chrome): () => Promise<boolean> {
  if (__ASSINADOR_ALVO__ !== 'firefox') return async () => true;
  return async () => {
    try {
      const permissoes = api.permissions as unknown as { contains(p: { data_collection: string[] }): Promise<boolean> };
      return await permissoes.contains({ data_collection: ['technicalAndInteraction'] });
    } catch {
      return false;
    }
  };
}

function remetenteDe(s: chrome.runtime.MessageSender | undefined): Remetente {
  return { id: s?.id, origin: s?.origin, url: s?.url, frameId: s?.frameId, tab: s?.tab ? { id: s.tab.id, windowId: s.tab.windowId } : undefined };
}

// No navegador, liga o fundo às APIs. Nos testes não há API de extensão, e nada roda.
const api = navegador();
if (api) {
  const fundo = criarFundo({
    idDaExtensao: api.runtime.id,
    urlDaExtensao: api.runtime.getURL(''),
    versao: api.runtime.getManifest().version,
    dev: __ASSINADOR_DEV__,
    navegador: navigator.userAgent,
    conectar: () => portaDoNavegador(api),
    armazenamento: armazenamentoLocal(api),
    janelas: criarJanelas(controleDoNavegador(api), (pagina) => api.runtime.getURL(pagina), () => crypto.randomUUID(), relogioReal),
    novoId: () => crypto.randomUUID(),
    relogio: relogioReal,
    podeEntregarDiagnostico: consentimentoDoDiagnostico(api),
  });
  // A página: uma porta por pedido, um pedido por porta. A porta que cai é a página que saiu.
  api.runtime.onConnect.addListener((porta) => {
    if (porta.name !== PORTA_DO_PEDIDO) {
      porta.disconnect();
      return;
    }
    let atendido = false;
    let avisarSaida: () => void = () => undefined;
    const saiu = new Promise<void>((resolver) => {
      avisarSaida = resolver;
    });
    porta.onDisconnect.addListener(() => avisarSaida());
    porta.onMessage.addListener((mensagem: unknown) => {
      if (atendido) return;
      atendido = true;
      void fundo.atenderPagina(mensagem, remetenteDe(porta.sender), saiu).then((resultado) => {
        try {
          porta.postMessage(resultado);
        } catch {
          // A página já saiu.
        }
      });
    });
  });
  // As páginas da própria extensão. `sendResponse` com `return true`, e não promessa devolvida: é a
  // forma que o Chrome e o Firefox aceitam os dois.
  api.runtime.onMessage.addListener((mensagem: unknown, remetente: chrome.runtime.MessageSender, responder: (r: unknown) => void) => {
    fundo.atenderExtensao(mensagem, remetenteDe(remetente)).then(responder, () => responder(null));
    return true;
  });
}
