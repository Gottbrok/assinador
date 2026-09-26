/**
 * `opcoes.html`: os endereços que a pessoa permitiu (com "Remover" em cada um), as versões da
 * extensão e do programa, e o diagnóstico para o suporte (§4 do plano).
 *
 * A lista é lida e escrita por `permissoes.ts`, o único escritor da chave. As versões e o
 * diagnóstico passam pelo fundo, que é quem fala com o programa; o diagnóstico vale mesmo sem
 * endereço autorizado (é o primeiro gesto do suporte) e não tem CPF.
 */

import { navegador } from './api';
import { aplicarTextos, dataParaLer, idiomaDoNavegador, tradutorDoNavegador, type Traduzir } from './i18n';
import { armazenamentoLocal, CHAVE_DAS_PERMISSOES, listarPermissoes, revogar, type Armazenamento } from './permissoes';

/** A linha do programa na seção de versões, a partir da resposta do `ola` (`versoes` no fundo). */
export function linhaDoPrograma(versoes: unknown, t: Traduzir): string {
  if (typeof versoes !== 'object' || versoes === null) return t('programaSemResposta');
  const { nativo, motivoNativo } = versoes as { nativo?: unknown; motivoNativo?: unknown };
  if (typeof nativo === 'object' && nativo !== null) {
    const { versao, plataforma } = nativo as { versao?: unknown; plataforma?: unknown };
    if (typeof versao === 'string' && typeof plataforma === 'string') return t('programaVersao', [versao, plataforma]);
  }
  return t(motivoNativo === 'ausente' ? 'programaAusente' : 'programaSemResposta');
}

/** O texto do diagnóstico, ou `null` com o código do erro. */
export function lerDiagnostico(resposta: unknown): { texto: string } | { codigo: string } {
  if (typeof resposta !== 'object' || resposta === null) return { codigo: 'interno' };
  const r = resposta as { ok?: unknown; dados?: { texto?: unknown }; erro?: { codigo?: unknown } };
  if (r.ok === true && typeof r.dados?.texto === 'string') return { texto: r.dados.texto };
  return { codigo: typeof r.erro?.codigo === 'string' ? r.erro.codigo : 'interno' };
}

function el<T extends HTMLElement>(id: string): T {
  const e = document.getElementById(id);
  if (!e) throw new Error(`#${id} ausente em opcoes.html`);
  return e as T;
}

async function desenharPermissoes(armazenamento: Armazenamento, t: Traduzir, idioma: string): Promise<void> {
  const lista = el<HTMLUListElement>('permissoes');
  const vazia = el<HTMLParagraphElement>('permissoes-vazia');
  const permissoes = await listarPermissoes(armazenamento);
  lista.replaceChildren();
  vazia.hidden = permissoes.length > 0;
  for (const p of permissoes) {
    const item = document.createElement('li');
    const origem = document.createElement('span');
    origem.className = 'origem';
    origem.textContent = p.origem;
    const desde = document.createElement('span');
    desde.className = 'secundario';
    desde.textContent = t('permitidoDesde', dataParaLer(p.desde, idioma, false));
    const remover = document.createElement('button');
    remover.type = 'button';
    remover.className = 'secundario';
    remover.textContent = t('botaoRemover');
    remover.setAttribute('aria-label', t('removerEndereco', p.origem));
    remover.addEventListener('click', () => {
      remover.disabled = true;
      void revogar(armazenamento, p.origem).then(() => desenharPermissoes(armazenamento, t, idioma));
    });
    const textos = document.createElement('div');
    textos.append(origem, desde);
    item.append(textos, remover);
    lista.append(item);
  }
}

async function iniciar(api: typeof chrome): Promise<void> {
  const t = tradutorDoNavegador(api);
  const idioma = idiomaDoNavegador(api);
  document.documentElement.lang = idioma;
  aplicarTextos(document, t);
  const armazenamento = armazenamentoLocal(api);
  const enviar = (m: unknown): Promise<unknown> => api.runtime.sendMessage(m);

  await desenharPermissoes(armazenamento, t, idioma);
  // A pessoa pode permitir um endereço com as opções abertas noutra aba.
  api.storage.onChanged.addListener((mudancas, area) => {
    if (area === 'local' && Object.hasOwn(mudancas, CHAVE_DAS_PERMISSOES)) void desenharPermissoes(armazenamento, t, idioma);
  });

  el('versao-extensao').textContent = t('extensaoVersao', api.runtime.getManifest().version);
  const programa = el('versao-programa');
  programa.textContent = t('procurandoPrograma');
  enviar({ tipo: 'versoes' }).then(
    (v) => {
      programa.textContent = linhaDoPrograma(v, t);
    },
    () => {
      programa.textContent = t('programaSemResposta');
    },
  );

  const gerar = el<HTMLButtonElement>('gerar-diagnostico');
  const copiar = el<HTMLButtonElement>('copiar-diagnostico');
  const saida = el<HTMLPreElement>('diagnostico');
  const aviso = el<HTMLParagraphElement>('aviso-diagnostico');
  gerar.addEventListener('click', () => {
    gerar.disabled = true;
    copiar.hidden = true;
    saida.hidden = true;
    aviso.textContent = t('gerandoDiagnostico');
    enviar({ tipo: 'diagnostico' })
      .then(
        (r) => {
          const d = lerDiagnostico(r);
          if ('texto' in d) {
            saida.textContent = d.texto;
            saida.hidden = false;
            copiar.hidden = false;
            aviso.textContent = t('diagnosticoSemCpf');
          } else {
            aviso.textContent = t('diagnosticoFalhou', d.codigo);
          }
        },
        () => {
          aviso.textContent = t('diagnosticoFalhou', 'interno');
        },
      )
      .finally(() => {
        gerar.disabled = false;
      });
  });
  copiar.addEventListener('click', () => {
    navigator.clipboard.writeText(saida.textContent ?? '').then(
      () => {
        aviso.textContent = t('diagnosticoCopiado');
      },
      () => {
        aviso.textContent = t('diagnosticoNaoCopiado');
      },
    );
  });
}

const api = navegador();
if (api && typeof document !== 'undefined') void iniciar(api);
