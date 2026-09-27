# Envio às lojas

O que as três lojas pedem (Chrome Web Store, Edge Add-ons e addons.mozilla.org) e os textos prontos
para colar, para o envio da F7a. Nome, descrição e permissões saem do código: o manifesto
(`extensao/manifest.base.ts`) e os `_locales` são a fonte, e este arquivo diz o que cada loja pergunta
além deles. Nada aqui afirma o que o código não faz; mudou o código, este arquivo, a política de
privacidade e o `extensao/PRIVACIDADE.md` mudam juntos.

## O que vale para as três

| Campo | Valor |
|---|---|
| Nome | **Assinador uShield** (decisão D2 do plano, 2026-09-27), o mesmo nas duas línguas; vem do `nomeDaExtensao` |
| Resumo | O `descricaoDaExtensao` de cada `_locales` (132 caracteres no máximo, conferido por teste) |
| Idiomas | Português do Brasil (padrão) e espanhol |
| Política de privacidade | `https://ushield.app/componente/privacidade` |
| Página de apoio | `https://ushield.app/componente` (instalar, conferir a instalação e copiar o diagnóstico) |
| Código-fonte | `https://github.com/Gottbrok/assinador` |
| Pacotes | `assinador-extensao-chrome.zip` (Chrome e Edge) e `assinador-extensao-firefox.zip`, os MESMOS bytes da release (conferidos pelo `SHA256SUMS`); nunca um zip montado à parte |

### Descrição detalhada, em português

> O Assinador uShield deixa você assinar com o certificado digital ICP-Brasil do seu cartão ou token
> nos sistemas do Confidata e do uShield.
>
> Ele tem duas partes, instaladas uma vez por computador: esta extensão e um programa pequeno, que lê o
> cartão pelo programa do fabricante (no Linux) ou pelo próprio Windows. A extensão sozinha não
> assina: sem o programa, ela diz o que falta e leva à página de instalação.
>
> Antes de cada assinatura, uma janela da extensão mostra o documento, o endereço que pede e o
> certificado escolhido, e só então pede o PIN. O programa só assina o resumo que o sistema daquele
> endereço preparou e assinou, e confere o endereço, o resumo, o certificado e o prazo antes de pedir a
> assinatura ao cartão. Um endereço só usa o Assinador depois da sua permissão, uma vez.
>
> A extensão funciona só em https://*.confidata.app e https://ushield.app. Nem ela nem o programa
> acessam a internet, e nenhum dos dois guarda o PIN, documentos ou certificados. O código é aberto.
>
> Para instalar o programa e conferir a instalação: https://ushield.app/componente

### Descrição detalhada, em espanhol

> El Assinador uShield te permite firmar con el certificado digital ICP-Brasil de tu tarjeta o token
> en los sistemas de Confidata y de uShield.
>
> Tiene dos partes, que se instalan una vez por computador: esta extensión y un programa pequeño, que
> lee la tarjeta a través del programa del fabricante (en Linux) o del propio Windows. La extensión
> sola no firma: sin el programa, dice lo que falta y lleva a la página de instalación.
>
> Antes de cada firma, una ventana de la extensión muestra el documento, la dirección que lo pide y el
> certificado elegido, y solo entonces pide el PIN. El programa solo firma el resumen que el sistema de
> esa dirección preparó y firmó, y revisa la dirección, el resumen, el certificado y el plazo antes de
> pedir la firma a la tarjeta. Una dirección solo usa el Assinador después de tu permiso, una vez.
>
> La extensión funciona solo en https://*.confidata.app y https://ushield.app. Ni ella ni el programa
> acceden a internet, y ninguno de los dos guarda el PIN, documentos ni certificados. El código es
> abierto.
>
> Para instalar el programa y revisar la instalación: https://ushield.app/componente

### Notas para quem revisa (Edge e Firefox pedem; em inglês)

> This extension only works together with a native program ("Assinador") installed on the computer
> and a smart card or USB token holding a Brazilian ICP-Brasil certificate. It talks to the program
> through native messaging and never accesses the network.
>
> It runs only on https://*.confidata.app and https://ushield.app. To see it without a card: install
> the extension, open https://ushield.app/componente and click "Conferir agora" (check now). Without
> the native program, the page says that the program is missing, and the first request from the page
> opens the extension's own permission window for that address. The native program for Windows and
> Linux is in the GitHub release of the same version (https://github.com/Gottbrok/assinador/releases);
> with it installed, the check confirms the installation. The options page lists the addresses the
> user has allowed, each with a "Remover" (remove) button, and "Gerar diagnóstico" shows the readers,
> card programs and certificates the program finds.
>
> A signature needs a request prepared and signed by the server of the allowed address (a short-lived
> ticket that the native program verifies) and the user's confirmation in the extension's own window,
> which shows the document, the requesting address and the chosen certificate before asking for the
> PIN. The scripts are bundled by esbuild but not minified; the Firefox source build instructions are
> below.

## Chrome Web Store

**Finalidade única:**

> Assinar, com o certificado digital ICP-Brasil do cartão ou token da pessoa, os documentos que os
> sistemas do Confidata e do uShield pedem, mostrando antes o que vai ser assinado.

**Justificativa de cada permissão:**

| Permissão | Justificativa |
|---|---|
| `nativeMessaging` | Falar com o programa Assinador instalado no computador, que lê o certificado e pede a assinatura ao cartão ou token: o navegador não alcança o cartão sozinho. |
| `storage` | Guardar, só neste computador (`storage.local`, nunca sincronizado), a lista de endereços que a pessoa permitiu. |
| Acesso aos sites (o script de conteúdo em `https://*.confidata.app/*` e `https://ushield.app/*`) | Receber o pedido de assinatura da página desses sistemas, e só dele: o script não lê o conteúdo da página nem o que a pessoa digita, e não entra em nenhum outro site. |

**Código remoto:** não. Todo o código está no pacote, e a extensão não carrega nem executa código de
fora.

**Uso de dados** (a aba de práticas de privacidade):

| Tipo | Marcar | Por quê |
|---|---|---|
| Informações de identificação pessoal | Sim | A extensão entrega à página que a pessoa autorizou os certificados do computador, com o nome e o CPF ou o CNPJ do titular e, no e-CPF, a data de nascimento. É a finalidade da extensão |
| Informações de autenticação | Sim, por cautela | No Linux, o PIN passa pela janela da extensão até o programa local. Ele não sai do computador nem é guardado, mas é um PIN que a extensão toca, e a loja o lista nesta categoria |
| Os demais (saúde, finanças, comunicações, localização, histórico, atividade, conteúdo de site) | Não | A extensão não os acessa |

As três certificações da aba (não vender os dados, não usá-los para fim alheio à finalidade única,
não usá-los para crédito) valem, e se marcam.

**Visibilidade:** decisão do Cairo no envio. Pública, a extensão aparece na busca da loja; não
listada, só quem tem o link a acha (a tela de instalação leva a ele). A extensão só funciona nos dois
endereços, então a busca pouco ajuda a quem não veio por eles.

## Edge Add-ons

Os mesmos textos da Chrome Web Store: a descrição, a política de privacidade, a página de apoio e as
notas para quem revisa (o campo "Notes for certification"). O pacote é o `assinador-extensao-chrome.zip`.
O Edge dá à extensão um ID PRÓPRIO, diferente do da Chrome Web Store: os dois entram em
`origem.ExtensoesChrome()` do programa (os manifestos de Chrome, Chromium e Edge são um só).

## addons.mozilla.org

- **ID:** `assinador@confidata.com.br`, fixo no manifesto (nome interno; não muda com a D2).
- **Compatibilidade:** só Firefox para computador, a partir do 140 (ESR). No Android não há native
  messaging: desmarcar o Android no envio. O `web-ext lint` deixa um aviso, de propósito: o
  `data_collection_permissions` pede o Firefox 142 no Android.
- **Coleta de dados:** está declarada no manifesto (`data_collection_permissions`): o dado de
  identificação do certificado é obrigatório, e o diagnóstico (dado técnico) é opcional, com o
  consentimento da pessoa na instalação ou nas opções. A loja mostra isso na instalação.
- **Fontes:** a loja pede o código-fonte de pacote gerado por build. Vai o arquivo de fontes da tag
  (o "Source code" que o GitHub anexa à release, ou `git archive v<versão>`), e estas instruções para
  quem revisa:

> The package is built from the `extensao/` folder of this source archive. The scripts are bundled by
> esbuild, one file per entry point, without minification.
>
> Environment: Linux (the release is built on Ubuntu 24.04), Node.js 22 (22.18 or newer), npm 10,
> and Info-ZIP `zip` 3.0.
>
> ```
> cd extensao
> npm ci --ignore-scripts
> SOURCE_DATE_EPOCH=<value from the release notes> node build.mjs --zip
> sha256sum pacotes/assinador-extensao-firefox.zip
> ```
>
> The SHA-256 is the one of the uploaded package (also listed in the SHA256SUMS of the GitHub
> release). `SOURCE_DATE_EPOCH` fixes the file dates inside the zip: without a git history (as in a
> source archive), the build uses it, and a different value gives a different zip. Unzipped, the
> package is `extensao/dist/firefox`.

## Antes de enviar

- [ ] O deploy do ushield com `https://ushield.app/componente/privacidade` no ar (a página existe
      desde a F7a; as lojas abrem o endereço na revisão).
- [ ] O encarregado de dados configurado no ushield (`USHIELD_ENCARREGADO_NOME` e
      `USHIELD_ENCARREGADO_EMAIL`): sem ele, a política diz que o contato "será publicado", e as lojas
      esperam um contato na política.
- [ ] Os ícones definitivos, com a marca do uShield: os de `extensao/icones/` são PROVISÓRIOS
      (`scripts/gerar-icones-provisorios.mjs`). A página do item nas lojas também pede o ícone (o de
      128 px).
- [ ] As capturas de tela, com os ícones definitivos e o programa de release: a janela de permissão, a
      de confirmação e as opções. A Chrome Web Store pede de uma a cinco, em 1280 x 800 ou 640 x 400,
      e o bloco promocional pequeno, de 440 x 280.
- [ ] As contas de desenvolvedor das três lojas e o e-mail de contato delas (atos do Cairo).
- [ ] Os rascunhos na Chrome Web Store e no Edge, que dão os dois IDs: eles entram em
      `origem.ExtensoesChrome()`, e só então o programa e os manifestos de produção saem (sem os IDs,
      o gerador de manifestos recusa, e a release para).
- [ ] O zip enviado é o da release, e o `SHA256SUMS` confere com ele.
