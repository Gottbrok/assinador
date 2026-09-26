# Privacidade da extensão Assinador

A extensão existe para uma coisa: deixar que um endereço que você autorizou peça uma assinatura com
o certificado digital do seu cartão ou token, e mostrar a você, antes, o que vai ser assinado.

## O que ela acessa

- **Os endereços do Confidata e do ushield** (`https://*.confidata.app` e `https://ushield.app`).
  Nesses endereços a extensão só escuta os pedidos que a página faz a ela. Ela não lê o conteúdo da
  página, não lê o que você digita nela e não roda em nenhum outro site.
- **O programa Assinador instalado no seu computador**, por native messaging. É ele que lê os
  certificados e assina com o cartão; a extensão só repassa o pedido e a resposta.
- **A lista de endereços que você permitiu**, guardada neste computador (`storage.local`). Ela não é
  sincronizada entre computadores e pode ser apagada, endereço por endereço, nas opções da extensão.
- **O PIN do cartão**, só quando você o digita na janela de confirmação e só pelo tempo de repassá-lo
  ao programa. A extensão não o guarda, e o programa o apaga da memória depois de usar.

## O que ela não faz

- Não acessa a internet. Não envia nada a servidor nenhum, nosso ou de terceiros.
- Não coleta estatística, histórico de navegação nem dado de uso.
- Não guarda documentos, assinaturas, certificados nem PIN.
- Não deixa um endereço pedir assinatura sem você: cada endereço precisa da sua permissão uma vez,
  e cada assinatura passa pela janela de confirmação, com o documento, o endereço e o certificado
  na tela.

## O que a página que pediu recebe

A lista dos certificados do computador (depois da sua permissão para aquele endereço), a assinatura
do resumo do documento (depois da sua confirmação) e, se você pedir, o diagnóstico para o suporte,
que não tem número de documento.

O código da extensão e do programa é aberto, em `https://github.com/Gottbrok/assinador`, e o pacote
publicado pode ser refeito a partir do código e conferido byte a byte.
