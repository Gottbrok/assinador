# Privacidade do Assinador uShield

O texto público, o que as lojas de extensão publicam, está em
`https://ushield.app/componente/privacidade` (no repositório do ushield,
`src/content/privacidadeDoAssinador.ts`). Este arquivo diz o mesmo para quem lê o código, e cada frase
tem o código que a sustenta: mudou o que a extensão ou o programa acessam ou entregam, os dois textos
mudam juntos, e a data da página também.

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

- **Os certificados que servem para assinar:** no Linux, os do seu cartão ou token, pelo programa do
  fabricante do cartão (módulo PKCS#11); no Windows, pelo próprio Windows, os do cartão e todos os que
  estão instalados no seu usuário (`CurrentUser\My`), inclusive os de outras pessoas ou empresas que
  estejam instalados ali.
- **O estado das leitoras de cartão e a identificação do tipo de cartão** (o ATR), só para o
  diagnóstico dizer o que falta quando algo não funciona. O programa nunca manda comando ao cartão
  pelo PC/SC.
- **A chave de cada certificado fica onde está:** no cartão, no token ou no repositório de
  certificados do Windows. O programa pede a assinatura a quem guarda a chave, e nunca a copia.

## O PIN

- No Linux, você digita o PIN na janela de confirmação da extensão; ela o repassa ao programa, que o
  entrega ao programa do fabricante do cartão e o apaga da memória em seguida. Com leitora de teclado,
  ou quando o programa do fabricante tem diálogo próprio (`CKF_PROTECTED_AUTHENTICATION_PATH`), você
  digita o PIN nelas: ele não passa pela extensão, e o programa Assinador não o recebe.
- No Windows, você digita o PIN num diálogo do próprio Windows ou do programa do fabricante do cartão:
  ele não passa pela extensão, e o programa Assinador não o recebe (o diálogo é do provedor do cartão,
  que o programa só chama para assinar).
- Nenhum dos dois guarda o PIN, e nenhum dos dois o repete sozinho depois de um erro: cada tentativa é
  sua.

## O que a página que você autorizou recebe

- **Sem pedir nada a você** (o `ola`): que a extensão está instalada, se o programa está instalado, as
  versões dos dois e o sistema e a arquitetura do computador (por exemplo, `windows-amd64`), para saber
  o que falta instalar ou atualizar.
- **Depois da sua permissão para aquele endereço** (o `listar`): todos os certificados do computador
  que servem para assinar, cada um inteiro (o DER), com tudo o que a certificadora gravou nele. No
  e-CPF, isso inclui o nome, o CPF e a data de nascimento do titular e, conforme o certificado, o NIS,
  o RG, o título de eleitor e o e-mail. No e-CNPJ, o nome e o CNPJ da empresa e o nome, o CPF e a data
  de nascimento do responsável. Junto de cada certificado vão o nome da leitora (que pode trazer o
  número de série dela), o programa do cartão que o leu e o estado do PIN, se restam poucas tentativas
  ou se o cartão está bloqueado.
- **Depois da sua confirmação na janela**, que mostra o nome do documento, o endereço e o certificado:
  a assinatura. O que se assina é um resumo que o servidor do endereço preparou: de um documento, que
  não passa pela extensão nem pelo programa, ou, na verificação de identidade, de um código que ele
  gerou.
- **Se a página pedir, o diagnóstico para o suporte:** as versões, o sistema, a identificação do
  navegador (o `userAgent`, a mesma que ele dá a qualquer site), as leitoras e o tipo de cartão, os
  programas de cartão com o lugar de cada um no computador, o estado dos serviços de cartão do sistema,
  e os certificados com o nome do titular, o emissor e a validade, com os dígitos de CPF e CNPJ
  trocados por asteriscos (`assinatura.Mascarar` troca só os dígitos: o nome sai inteiro). No Firefox
  isso está declarado à loja: o dado de identificação do certificado é obrigatório (é para isso que a
  extensão existe), e o diagnóstico é opcional, só vai à página com o seu consentimento, na instalação
  ou nas opções da extensão. No Chrome e no Edge, basta a permissão do endereço.

O que a página faz com o que recebe é regido pela política de privacidade do serviço dela.

## O que nenhum dos dois faz

- O código da extensão e o do programa não abrem conexão com servidor nenhum, nem nosso nem de
  terceiros (o build da extensão reprova rede no pacote, e o programa não tem cliente de rede). O
  programa do fabricante do cartão, que o Assinador usa para falar com o cartão, é de terceiros e segue
  a política do fabricante: no Windows ele roda dentro do processo do programa, e no Linux, num processo
  filho dele.
- Não coletam estatística, histórico de navegação nem dado de uso, e não guardam documentos,
  assinaturas, certificados nem PIN.
- Não assinam sem você nem fora do que foi pedido. Cada endereço precisa da sua permissão uma vez,
  cada assinatura passa pela janela de confirmação, e o programa só assina o resumo que o servidor
  daquele endereço preparou e assinou: antes de pedir a assinatura, ele confere o endereço, o resumo, o
  certificado e o prazo (o bilhete, `protocolo/PROTOCOLO.md`).

## Para remover

Remova a extensão pela página de extensões do navegador, e o programa como qualquer outro programa do
computador. A lista de endereços permitidos sai junto com a extensão.

## Código aberto

O código da extensão e do programa é aberto, em `https://github.com/Gottbrok/assinador`, e o pacote
publicado de cada versão pode ser refeito a partir do código dela, com o `SOURCE_DATE_EPOCH` publicado
nas notas da release, e conferido byte a byte.
