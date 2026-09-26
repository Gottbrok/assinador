// A página de teste da extensão. Fala o protocolo da página (canal, versão, id, prazos) à mão, com
// as mesmas regras da `ponteDaJanela` da biblioteca `@confidata/icp-brasil`: este repositório é
// público e não depende da biblioteca, que é privada. O teste ponta a ponta usa `window.assinador`;
// a pessoa, os botões.
'use strict';

(() => {
  const CANAL_PEDIDO = 'assinador:pedido';
  const CANAL_RESPOSTA = 'assinador:resposta';
  const PROTOCOLO = 1;
  const PRAZOS = { ola: 1500, listar: 30000, diagnostico: 30000, assinar: 240000 };

  /** Um pedido à extensão; `null` quando o prazo acaba sem resposta. */
  function pedir(op, dados) {
    return new Promise((resolve) => {
      const id = crypto.randomUUID();
      const origem = window.location.origin;
      let relogio;
      const ouvir = (evento) => {
        if (evento.source !== window || evento.origin !== origem) return;
        const m = evento.data;
        if (!m || typeof m !== 'object' || m.canal !== CANAL_RESPOSTA || m.id !== id) return;
        clearTimeout(relogio);
        window.removeEventListener('message', ouvir);
        if (m.v !== PROTOCOLO) resolve({ ok: false, erro: { codigo: 'protocolo', detalhe: 'versão' } });
        else resolve(m.ok ? { ok: true, dados: m.dados } : { ok: false, erro: m.erro });
      };
      window.addEventListener('message', ouvir);
      relogio = setTimeout(() => {
        window.removeEventListener('message', ouvir);
        resolve(null);
      }, PRAZOS[op]);
      window.postMessage({ canal: CANAL_PEDIDO, v: PROTOCOLO, id, op, ...(dados ? { dados } : {}) }, origem);
    });
  }

  async function postar(caminho, corpo) {
    const r = await fetch(caminho, { method: 'POST', headers: { 'content-type': 'application/json' }, body: JSON.stringify(corpo) });
    const json = await r.json();
    if (!r.ok) throw new Error(json.erro || `HTTP ${r.status}`);
    return json;
  }

  /** O resumo (SHA-256) de um "documento" de teste, único a cada chamada. */
  async function resumo(texto) {
    const bytes = new TextEncoder().encode(`${texto}\n${new Date().toISOString()}\n${crypto.randomUUID()}`);
    const h = new Uint8Array(await crypto.subtle.digest('SHA-256', bytes));
    return Array.from(h, (b) => b.toString(16).padStart(2, '0')).join('');
  }

  const bilhete = async (ref, digest, documento) => (await postar('/bilhete', { ref, digest, documento })).bilhete;
  const conferir = async (der, digest, assinatura) => postar('/conferir', { der, digest, assinatura });

  /** O fluxo inteiro, como o Confidata o faz: resumo, bilhete, assinar, e o servidor confere. */
  async function assinarCom(certificado, documento) {
    const digest = await resumo(documento);
    const jws = await bilhete(certificado.ref, digest, documento);
    const resposta = await pedir('assinar', { ref: certificado.ref, digest, bilhete: jws });
    if (!resposta || !resposta.ok) return { resposta };
    return { resposta, conferencia: await conferir(certificado.der, digest, resposta.dados.assinatura) };
  }

  window.assinador = { pedir, bilhete, conferir, resumo, assinarCom };

  // A interface da pessoa.
  const saida = document.getElementById('saida');
  const lista = document.getElementById('certificados');
  let certificados = [];
  const mostrar = (valor) => {
    saida.textContent = JSON.stringify(valor, null, 2);
  };

  document.getElementById('ola').addEventListener('click', async () => mostrar(await pedir('ola')));
  document.getElementById('diagnostico').addEventListener('click', async () => {
    const r = await pedir('diagnostico');
    saida.textContent = r && r.ok ? r.dados.texto : JSON.stringify(r, null, 2);
  });
  document.getElementById('listar').addEventListener('click', async () => {
    lista.textContent = 'Procurando...';
    const r = await pedir('listar');
    mostrar(r);
    certificados = r && r.ok ? r.dados.certificados : [];
    lista.replaceChildren();
    if (certificados.length === 0) lista.textContent = 'Nenhum certificado.';
    certificados.forEach((c, i) => {
      const rotulo = document.createElement('label');
      const radio = document.createElement('input');
      radio.type = 'radio';
      radio.name = 'certificado';
      radio.value = String(i);
      radio.checked = i === 0;
      rotulo.append(radio, ` ${c.rotuloDoProvedor} · PIN ${c.exigePin ? 'na janela' : 'no dispositivo'}${c.estadoDoPin ? ` (${c.estadoDoPin})` : ''} · ref ${c.ref.slice(0, 12)}...`);
      lista.append(rotulo);
    });
  });
  document.getElementById('assinar').addEventListener('click', async () => {
    const escolhido = document.querySelector('input[name="certificado"]:checked');
    const certificado = escolhido ? certificados[Number(escolhido.value)] : undefined;
    if (!certificado) {
      saida.textContent = 'Procure os certificados e escolha um.';
      return;
    }
    saida.textContent = 'Confirme na janela do Assinador.';
    try {
      mostrar(await assinarCom(certificado, document.getElementById('documento').value));
    } catch (erro) {
      saida.textContent = `Falhou: ${erro.message}`;
    }
  });
})();
