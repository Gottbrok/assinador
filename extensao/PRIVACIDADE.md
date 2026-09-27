# Privacidade do Assinador uShield

O texto público, o que as lojas de extensão publicam, está em
`https://ushield.app/componente/privacidade` (no repositório do ushield,
`src/content/privacidadeDoAssinador.ts`). Este arquivo diz o mesmo para quem lê o código: mudou o que a
extensão ou o programa acessam, os dois textos mudam juntos, e a data da página também.

O Assinador uShield é uma extensão de navegador e um programa, que trabalham juntos para uma coisa só:
deixar que um endereço que você autorizou peça uma assinatura com o certificado digital do seu cartão
ou token, e mostrar a você, antes, o que vai ser assinado.

## O que a extensão acessa

- **Os endereços do Confidata e do uShield** (`https://*.confidata.app` e `https://ushield.app`).
  Neles, ela só escuta os pedidos que a página faz a ela: não lê o conteúdo da página, não lê o que
  você digita e não roda em nenhum outro site.
- **A lista dos endereços que você permitiu**, guardada só neste computador (`storage.local`). Ela não
  é sincronizada entre computadores, e você remove qualquer endereço nas opções da extensão.
- **O programa Assinador instalado no computador**, por native messaging, para repassar a ele cada
  pedido e trazer de volta a resposta.

## O que o programa acessa

- **Os certificados do seu cartão ou token:** no Linux, pelo programa do fabricante do cartão (módulo
  PKCS#11); no Windows, pelo próprio Windows, que mostra também os certificados instalados no seu
  usuário.
- **O estado das leitoras de cartão e a identificação do tipo de cartão** (o ATR), só para o
  diagnóstico dizer o que falta quando algo não funciona. O programa nunca manda comando ao cartão
  pelo PC/SC.
- **A chave do certificado continua no cartão ou no token:** o programa pede a ele que assine, e ela
  nunca sai de lá.

## O PIN

No Linux, você digita o PIN na janela de confirmação da extensão; ela o repassa ao programa, que o
entrega ao programa do fabricante do cartão e o apaga da memória em seguida. No Windows, você digita o
PIN num diálogo do próprio Windows ou do programa do fabricante do cartão: ele não passa pela
extensão, e o programa Assinador não o recebe (o diálogo é do provedor do cartão, que o programa só
chama para assinar). Nenhum dos dois guarda o PIN, e nenhum dos dois o repete sozinho depois de um
erro: cada tentativa é sua.

## O que a página que você autorizou recebe

- Sem pedir nada a você, só as versões da extensão e do programa (o `ola`), para saber se precisam de
  atualização.
- Depois da sua permissão para aquele endereço, os certificados do computador, cada um inteiro, com o
  que ele traz: o nome e o CPF ou o CNPJ do titular e, no e-CPF, a data de nascimento.
- Depois da sua confirmação na janela, que mostra o documento, o endereço e o certificado, a
  assinatura do resumo daquele documento. O documento em si não passa pela extensão nem pelo programa.
- Se a página pedir, o diagnóstico para o suporte: as versões, o sistema, as leitoras, os programas de
  cartão e os certificados com o nome mascarado, sem número de documento. No Firefox isso está
  declarado à loja: o dado de identificação do certificado é obrigatório (é para isso que a extensão
  existe), e o diagnóstico é opcional, só vai à página com o seu consentimento, na instalação ou nas
  opções da extensão.

O que a página faz com o que recebe é regido pela política de privacidade do serviço dela.

## O que nenhum dos dois faz

- Não acessam a internet: não abrem conexão com servidor nenhum, nosso ou de terceiros.
- Não coletam estatística, histórico de navegação nem dado de uso, e não guardam documentos,
  assinaturas, certificados nem PIN.
- Não assinam sem você nem fora do que foi pedido. Cada endereço precisa da sua permissão uma vez,
  cada assinatura passa pela janela de confirmação, e o programa só assina o resumo que o servidor
  daquele endereço preparou e assinou: antes de pedir a assinatura ao cartão, ele confere o endereço,
  o resumo, o certificado e o prazo (o bilhete, `protocolo/PROTOCOLO.md`).

## Para remover

Remova a extensão pela página de extensões do navegador, e o programa como qualquer outro programa do
computador. A lista de endereços permitidos sai junto com a extensão.

## Código aberto

O código da extensão e do programa é aberto, em `https://github.com/Gottbrok/assinador`, e o pacote
publicado de cada versão pode ser refeito a partir do código dela, com o `SOURCE_DATE_EPOCH` publicado
nas notas da release, e conferido byte a byte.
