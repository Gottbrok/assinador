// Página da extensão de prova: manda uma mensagem ao host e mostra o que voltou, com o navegador.
const HOST = 'br.com.confidata.assinador.prova';
const saida = document.getElementById('saida');
const resultados = [];

function registrar(rotulo, valor) {
  resultados.push({ rotulo, valor });
  saida.textContent = resultados.map((r) => `${r.rotulo}\n${JSON.stringify(r.valor, null, 2)}`).join('\n\n');
}

function enviar(op) {
  const inicio = performance.now();
  chrome.runtime.sendNativeMessage(HOST, { op }, (resposta) => {
    const ms = Math.round(performance.now() - inicio);
    const erro = chrome.runtime.lastError;
    registrar(`${op} (${ms} ms)`, erro ? { falhou: erro.message } : resposta);
  });
}

registrar('navegador', { userAgent: navigator.userAgent });
document.getElementById('ola').addEventListener('click', () => enviar('ola'));
document.getElementById('listar').addEventListener('click', () => enviar('listar'));
document.getElementById('copiar').addEventListener('click', () => navigator.clipboard.writeText(saida.textContent));
