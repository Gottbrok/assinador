/**
 * O fundo da extensão (service worker no Chrome e no Edge, script de fundo no Firefox).
 *
 * Recebe o pedido que o script de conteúdo repassou e o atende, em quatro portões, nesta ordem:
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
 * Um fluxo de assinatura por vez: o segundo recebe `ocupado`.
 *
 * Cada operação da página tem um ORÇAMENTO (`PRAZOS_DO_FLUXO_MS`), abaixo do prazo da página, e
 * cada passo usa o menor entre o teto dele e o que resta: a página recebe a resposta da extensão,
 * nunca desiste com uma janela ainda aberta.
 */

import { navegador } from './api';
import { controleDoNavegador, criarJanelas, type Janelas } from './janelas';
import { abrirPrograma, portaDoNavegador, relogioReal, type PortaNativa, type Programa, type Relogio } from './nativo';
import { hostDaOrigem, origemAceita, origemDoRemetente, type Remetente } from './origem';
import { armazenamentoLocal, permitido, permitir, type Armazenamento } from './permissoes';
import {
  falha,
  FORMA_DO_BILHETE,
  FORMA_HEX64,
  OPERACOES_DA_PAGINA,
  ORIGEM_DA_EXTENSAO,
  PRAZO_DA_DECISAO_MS,
  PRAZO_DA_PERMISSAO_MS,
  PRAZOS_DO_FLUXO_MS,
  RESERVA_DEPOIS_DA_CONFIRMACAO_MS,
  RESERVA_DEPOIS_DA_PERMISSAO_MS,
  TAMANHO_MAXIMO_DO_BILHETE,
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

function ehObjeto(valor: unknown): valor is Record<string, unknown> {
  return typeof valor === 'object' && valor !== null && !Array.isArray(valor);
}

function ehOperacao(op: unknown): op is OperacaoDaPagina {
  return typeof op === 'string' && (OPERACOES_DA_PAGINA as readonly string[]).includes(op);
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
  /** Um pedido vindo do script de conteúdo. Nunca rejeita. */
  atenderPagina(mensagem: unknown, remetente: Remetente): Promise<Resultado>;
  /** Uma mensagem vinda de uma página da própria extensão (janela ou opções). */
  atenderExtensao(mensagem: unknown, remetente: Remetente): Promise<unknown>;
}

export function criarFundo(amb: AmbienteDoFundo): Fundo {
  let assinando = false;
  const permissoesPendentes = new Map<string, Promise<Resultado | null>>();

  const programa = (): Programa => abrirPrograma(amb.conectar, amb.novoId, amb.relogio);

  /** O que resta do orçamento (`limite` é um instante de `relogio.agora()`); sem limite, sem teto. */
  const resta = (limite: number | null): number => (limite === null ? Number.POSITIVE_INFINITY : limite - amb.relogio.agora());

  async function umPedido(op: 'ola' | 'listar' | 'diagnostico', origem: string, limite: number | null): Promise<Resultado> {
    const p = programa();
    try {
      return await p.pedir(op, origem, undefined, resta(limite));
    } finally {
      p.fechar();
    }
  }

  async function ola(origem: string, limite: number | null): Promise<Resultado> {
    const r = await umPedido('ola', origem, limite);
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

  /** `null` quando a origem pode seguir; senão, a recusa (`permissao-negada`, ou `interno` se a janela nem abriu). */
  async function temPermissao(origem: string, limite: number): Promise<Resultado | null> {
    if (await permitido(amb.armazenamento, origem)) return null;
    // Dois pedidos do mesmo endereço ao mesmo tempo (listar e diagnóstico) abrem UMA janela.
    const pendente = permissoesPendentes.get(origem);
    if (pendente) return pendente;
    const prazo = Math.min(PRAZO_DA_PERMISSAO_MS, resta(limite) - RESERVA_DEPOIS_DA_PERMISSAO_MS);
    if (prazo <= 0) return falha('tempo-esgotado', 'sem tempo para pedir a permissão');
    const pergunta = (async (): Promise<Resultado | null> => {
      const d = await amb.janelas.esperar<unknown>('permitir.html', 420, 320, { host: hostDaOrigem(origem) }, prazo);
      if (d.tipo === 'falhou') return falha('interno', 'a janela de permissão não abriu');
      if (d.tipo === 'decisao' && ehObjeto(d.valor) && d.valor.permitir === true) {
        await permitir(amb.armazenamento, origem);
        return null;
      }
      return falha('permissao-negada');
    })();
    permissoesPendentes.set(origem, pergunta);
    try {
      return await pergunta;
    } finally {
      permissoesPendentes.delete(origem);
    }
  }

  async function diagnostico(origem: string, limite: number | null): Promise<Resultado> {
    const r = await umPedido('diagnostico', origem, limite);
    if (!r.ok) return r;
    const { relatorio, texto } = r.dados;
    if (!ehObjeto(relatorio) || typeof texto !== 'string') return falha('protocolo', 'diagnóstico sem forma');
    // O programa não sabe de navegador nem da extensão: quem acrescenta é quem sabe.
    return {
      ok: true,
      dados: {
        relatorio: { ...relatorio, extensao: { versao: amb.versao }, navegador: amb.navegador },
        texto: `Extensão: versão ${amb.versao}\nNavegador: ${amb.navegador}\n${texto}`,
      },
    };
  }

  async function assinar(origem: string, dadosBrutos: unknown, limite: number): Promise<Resultado> {
    const dados = lerDadosDoAssinar(dadosBrutos);
    if (!dados) return falha('protocolo', 'dados de assinar sem forma');
    if (assinando) return falha('ocupado', 'outra assinatura em curso');
    assinando = true;
    const p = programa();
    try {
      const conferido = await p.pedir('conferir', origem, dados, resta(limite));
      if (!conferido.ok) return conferido;
      const janela = lerConferir(conferido.dados, hostDaOrigem(origem));
      if (!janela) return falha('protocolo', 'conferir sem forma');
      if (janela.estadoDoPin === 'bloqueado') return falha('token-bloqueado', 'o token já está bloqueado');
      const prazo = Math.min(PRAZO_DA_DECISAO_MS, resta(limite) - RESERVA_DEPOIS_DA_CONFIRMACAO_MS);
      if (prazo <= 0) return falha('tempo-esgotado', 'sem tempo para a janela de confirmação');
      const d = await amb.janelas.esperar<unknown>('confirmar.html', 420, 560, janela, prazo);
      if (d.tipo === 'falhou') return falha('interno', 'a janela de confirmação não abriu');
      if (d.tipo === 'prazo') return falha('tempo-esgotado', 'a janela de confirmação não foi respondida');
      if (d.tipo === 'fechada') return falha('cancelado', 'a janela de confirmação foi fechada');
      const decisao = lerDecisao(d.valor);
      if (!decisao) return falha('protocolo', 'decisão sem forma');
      if (!decisao.assinar) return falha('cancelado', 'a pessoa cancelou na janela');
      if (janela.exigePin && !decisao.pin) return falha('protocolo', 'o dispositivo exige PIN e ele não veio');
      return await p.pedir('assinar', origem, janela.exigePin && decisao.pin ? { ...dados, pin: decisao.pin } : dados, resta(limite));
    } finally {
      p.fechar();
      assinando = false;
    }
  }

  return {
    async atenderPagina(mensagem, remetente) {
      try {
        if (remetente.id !== amb.idDaExtensao || remetente.frameId !== 0 || typeof remetente.tab?.id !== 'number') return falha('origem-recusada', 'remetente');
        if (!ehObjeto(mensagem) || mensagem.tipo !== 'pedido-da-pagina' || typeof mensagem.origem !== 'string') return falha('protocolo', 'mensagem sem forma');
        const origem = origemDoRemetente(remetente);
        if (origem === null || origem !== mensagem.origem || !origemAceita(origem, amb.dev)) return falha('origem-recusada', 'origem');
        if (!ehOperacao(mensagem.op)) return falha('protocolo', 'operação desconhecida');
        const limite = amb.relogio.agora() + PRAZOS_DO_FLUXO_MS[mensagem.op];
        if (mensagem.op === 'ola') return await ola(origem, limite);
        const recusa = await temPermissao(origem, limite);
        if (recusa) return recusa;
        switch (mensagem.op) {
          case 'listar':
            return await umPedido('listar', origem, limite);
          case 'diagnostico':
            return await diagnostico(origem, limite);
          case 'assinar':
            return await assinar(origem, mensagem.dados, limite);
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
          const r = await ola(ORIGEM_DA_EXTENSAO, null);
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
  });
  // `sendResponse` com `return true`, e não promessa devolvida: é a forma que o Chrome e o Firefox aceitam os dois.
  api.runtime.onMessage.addListener((mensagem: unknown, remetente: chrome.runtime.MessageSender, responder: (r: unknown) => void) => {
    const deUmaPagina = typeof mensagem === 'object' && mensagem !== null && (mensagem as { tipo?: unknown }).tipo === 'pedido-da-pagina';
    const r = { id: remetente.id, origin: remetente.origin, url: remetente.url, frameId: remetente.frameId, tab: remetente.tab ? { id: remetente.tab.id, windowId: remetente.tab.windowId } : undefined };
    (deUmaPagina ? fundo.atenderPagina(mensagem, r) : fundo.atenderExtensao(mensagem, r)).then(responder, () => responder(null));
    return true;
  });
}
