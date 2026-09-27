# Suporte do Assinador

Para quem atende o chamado de quem não consegue assinar com token ou cartão. Cada código de erro do
protocolo, cada frase que a página mostra sem código e cada aviso do diagnóstico, com a causa provável
e o que fazer. A coluna "Quem resolve" diz o que a própria pessoa resolve (é o que o artigo da Central
de Ajuda explica) e o que é do suporte ou da engenharia.

O que está aqui é o que o código faz hoje. O que ainda não foi medido com cartão real leva a marca
**(não medido)**, e a medição mora em `docs/medicoes/`.

## Nunca peça

- O PIN nem o PUK, por nenhum canal. O Assinador não os guarda e o suporte não precisa deles.
- O arquivo do certificado, a senha dele, ou foto do cartão.
- O CPF. O diagnóstico não tem CPF (o nome do titular sai com os dígitos trocados por `*`); se a
  pessoa mandar uma captura de tela com o CPF, descarte a captura.
- Que a pessoa apague certificado, do repositório do Windows ou do token. Apagar pode levar a chave
  junto, sem volta, e o certificado que parece velho pode ser o válido: o caso vai para a engenharia.

## O primeiro pedido: o diagnóstico

O diagnóstico é o relatório do Assinador para o suporte: o sistema, as leitoras e o cartão em cada
uma, os programas de cartão com o estado de cada um, os certificados (com o nome do titular e os
dígitos de CPF e CNPJ trocados por `*`) e os avisos em frase. Três jeitos de conseguir, do mais fácil
para o mais difícil:

1. **Pela extensão:** nas opções da extensão, "Gerar diagnóstico" e "Copiar diagnóstico". As opções
   abrem pela página de extensões do navegador: no Chrome e no Edge, "Extensões", o Assinador,
   "Detalhes", "Opções da extensão"; no Firefox, "Complementos e temas", o Assinador, "Opções".
   Funciona mesmo sem nenhum endereço autorizado e, no Firefox, sem o consentimento do envio às
   páginas.
2. **Pelo terminal, no Linux:** `/usr/lib/confidata-assinador/assinador diagnostico` (com `--json`,
   o mesmo relatório em JSON).
3. **Pelo terminal, no Windows:** `assinador.exe diagnostico`, na pasta do programa. A pasta está no
   manifesto que o navegador lê: `reg query HKCU\Software\Google\Chrome\NativeMessagingHosts\br.com.confidata.assinador`
   (na instalação por máquina, `HKLM` no lugar de `HKCU`) mostra o arquivo do manifesto, e o campo
   `path` dele é o programa.

Pelo terminal o diagnóstico roda sem o navegador: é o caminho quando a extensão não consegue falar com
o programa. Se a extensão diz que não consegue gerar o diagnóstico, o código entre parênteses diz por
quê: `nativo-ausente` é o programa que o navegador não acha, e `modulo-falhou`, o que ele acha e não
consegue abrir (veja `nativo-ausente` abaixo).

## Os códigos de erro

A página mostra a frase; o código e o detalhe técnico vêm junto, no erro que a página recebe. O
detalhe é texto para o suporte (no Linux, com a etapa e o `CKR_*` do programa do cartão; no Windows,
com a etapa e o código do Windows em hexadecimal; no bilhete, com a etapa da conferência, como
`etapa kid`) e nunca tem PIN nem CPF.

| Código | O que a pessoa lê | Causa provável | O que fazer | Quem resolve |
|---|---|---|---|---|
| `cancelado` | "Assinatura cancelada no Assinador." | A pessoa cancelou ou fechou a janela de confirmação, fechou o diálogo de PIN (no Windows, o do provedor do cartão), ou saiu da página no meio. Com o detalhe `embargo`, a página teve três janelas de confirmação recusadas (negadas, fechadas ou vencidas) em 10 minutos, e a extensão recusa por 2 minutos sem abrir janela. | Assinar de novo. Com `embargo`, esperar 2 minutos, ou fechar e abrir o navegador (o embargo mora na memória da extensão). | A pessoa |
| `permissao-negada` | "Você não permitiu que este endereço use o Assinador. A permissão se muda nas opções da extensão." | A pessoa negou ou fechou a janela de permissão do endereço. Com `embargo`, três janelas de permissão recusadas (negadas, fechadas ou vencidas) em 10 minutos, e o endereço fica 10 minutos sem janela. No Firefox, também o diagnóstico que a página pede com o envio às páginas desligado (o detalhe diz "o envio do diagnóstico às páginas está desligado"): ele nasce desligado, até a pessoa consentir na instalação ou nas opções. | Tentar de novo e clicar em "Permitir": a recusa não fica guardada, e a janela volta no próximo pedido. Com `embargo`, esperar 10 minutos, ou fechar e abrir o navegador. Para o diagnóstico no Firefox, marcar nas opções da extensão a caixa que permite o envio às páginas, ou gerar e copiar o diagnóstico pelas próprias opções. A frase manda mudar a permissão nas opções, mas as opções só listam os endereços PERMITIDOS, com "Remover": não há recusa para desfazer lá. | A pessoa |
| `pin-incorreto` | "PIN incorreto." Com "Restam poucas tentativas" ou "Resta UMA tentativa", quando o cartão informa. | O PIN digitado está errado. O Assinador nunca repete o PIN sozinho: cada erro é uma tentativa gasta. No Windows o Windows não informa quantas restam. | Conferir o PIN com calma antes de tentar de novo. Na última tentativa, parar e procurar o PUK antes (veja "PIN e PUK"). | A pessoa |
| `token-bloqueado` | "O cartão bloqueou por excesso de tentativas. O desbloqueio é com o PUK, pela sua certificadora." | O PIN foi errado vezes demais e o cartão bloqueou (ou já chegou bloqueado). No Linux, também o PIN VENCIDO (`CKR_PIN_EXPIRED` no detalhe): o cartão exige trocar o PIN antes de assinar, e não está bloqueado, apesar da frase. | Bloqueado: desbloquear com o PUK (veja "PIN e PUK"). Com `CKR_PIN_EXPIRED`: trocar o PIN pelo programa do fabricante (a opção de alterar o PIN), sem o PUK. O Assinador não desbloqueia cartão nem troca PIN. | A pessoa, com a certificadora |
| `nativo-ausente` | "A extensão está instalada. Falta o programa que lê o cartão." | Na detecção (o primeiro contato da página), o detalhe é `ausente` ou `falhou`. `ausente`: o programa não está instalado, ou o navegador não acha o manifesto dele. `falhou`: o programa está instalado e o navegador não o abre, ou ele não responde; o manifesto que não autoriza esta extensão (no Chrome e no Edge, "Access to the specified native messaging host is forbidden") é o pacote de DESENVOLVIMENTO com a extensão da loja, ou o contrário. Fora da detecção, o programa que o navegador não acha é `nativo-ausente` com a mensagem do navegador, e o que sai no meio de um pedido é `modulo-falhou`. No Linux, o navegador em Snap pode não conseguir abrir o programa (veja "Navegador em Snap", não medido). | `ausente`: instalar o programa pela tela de instalação e reabrir o navegador. `falhou`: conferir que o programa e a extensão são os de produção (a extensão da loja e o programa da tela de instalação, nunca um pacote de desenvolvimento), reinstalar o programa e reabrir o navegador; se repetir, o diagnóstico pelo terminal. No Linux com Snap, usar o navegador do pacote `.deb`. | A pessoa; o suporte se persistir |
| `nativo-desatualizado` | "Há uma versão nova do Assinador. Atualize para continuar." | Hoje nenhuma ponta o produz: está no vocabulário do protocolo, e quem confere a versão mínima é a biblioteca, que diz a mesma frase sem código (veja "Versão abaixo da mínima"). Se aparecer, veio de uma versão nova de alguma ponta. | Seguir "Versão abaixo da mínima"; se o código em si aparecer, é da engenharia. | A engenharia |
| `tempo-esgotado` | "O Assinador não recebeu resposta a tempo. Tente de novo." | Algum passo não terminou no prazo. No Linux, o programa do cartão não assinou em 90 s. O programa não respondeu no orçamento da extensão (100 s para assinar): no Windows, é o que acontece com o diálogo de PIN aberto e não respondido, porque o Windows não impõe prazo a ele. O dispositivo esteve ocupado o prazo inteiro. Ou a janela de permissão (25 s) ou de confirmação (180 s) ficou sem resposta. Na LISTAGEM, o programa do cartão que passa de 20 s não dá este erro: ele é deixado de lado, o certificado dele não aparece, e o diagnóstico diz que ele falhou. | Desconectar e reconectar o token ou o cartão, fechar outro programa que esteja usando o cartão, e tentar de novo, respondendo a janela e o diálogo de PIN sem demora. Se repetir, o diagnóstico mostra qual programa de cartão falhou. | A pessoa; o suporte se repetir |
| `modulo-falhou` | "O programa do cartão ou do token falhou. Reconecte o dispositivo e tente de novo." | No Linux, o programa do fabricante (módulo PKCS#11) caiu na assinatura ou recusou com um código que o protocolo não distingue (o detalhe traz a etapa e o `CKR_*`). No Windows, o provedor do cartão recusou: o detalhe traz a etapa e o código. `0x80090019` (`NTE_KEYSET_NOT_DEF`) é o provedor do fabricante que não está instalado; `0x80090010` (`NTE_PERM`) e `0x80070005` são acesso negado; `0x80004005` é recusa sem motivo. Também é o programa do Assinador que saiu no meio de um pedido (o detalhe traz a mensagem do navegador). | Reconectar e tentar de novo. Se repetir: o diagnóstico. Com `0x80090019`, instalar o programa do fabricante do cartão para Windows; nos outros casos, reinstalar o programa do fabricante e mandar o diagnóstico e o detalhe à engenharia. | O suporte |
| `ocupado` | "O Assinador está ocupado com outra operação. Conclua ou cancele a janela aberta e tente de novo." | Outra assinatura em curso (outra aba, outra janela de confirmação aberta), ou a fila do dispositivo cheia. | Concluir ou fechar a outra janela e tentar de novo. | A pessoa |
| `bilhete-invalido` | "A sessão de assinatura expirou. Procure os certificados de novo." | O pedido de assinatura que o sistema preparou não passou na conferência do programa. Com `etapa kid`, a chave do emissor é uma que o programa não conhece (programa velho para uma chave nova); nas outras etapas, bilhete malformado ou assinatura inválida. | Recarregar a página e procurar os certificados de novo. Com `etapa kid`, atualizar o programa; se ainda repetir, ou em qualquer outra etapa, é da engenharia (o programa e o sistema não se entendem). | O suporte; a engenharia |
| `bilhete-expirado` | "A sessão de assinatura expirou. Procure os certificados de novo." | Hoje nenhuma das pontas o produz: está no vocabulário do protocolo, e o bilhete fora do prazo sai como `relogio`. Se aparecer, veio de uma versão nova de alguma ponta. | Recarregar a página e procurar os certificados de novo; se repetir, é da engenharia. | A engenharia |
| `relogio` | "O relógio deste computador está errado. Acerte a data e a hora e tente de novo." | A hora do computador está fora da janela do pedido (a emissão menos 15 minutos até o vencimento mais 15 minutos). Quase sempre é relógio errado: dual boot, bateria do computador fraca, ou fuso trocado com a hora acertada à mão (a hora parece certa e o horário universal está errado). | Ligar o acerto automático de data e hora e conferir o fuso horário; depois recarregar a página. | A pessoa |
| `digest-divergente` | "A sessão de assinatura expirou. Procure os certificados de novo." | A página pediu para assinar um conteúdo diferente do que o sistema autorizou. | Recarregar e tentar de novo. Se repetir, é da engenharia, e é sinal de segurança (página alterada no caminho): registrar o endereço e a hora. | A engenharia |
| `certificado-divergente` | "A sessão de assinatura expirou. Procure os certificados de novo." | O certificado escolhido não é o que o sistema preparou (a pessoa trocou de token ou de certificado entre a busca e a assinatura). | Procurar os certificados de novo e escolher o certificado de novo. | A pessoa |
| `origem-recusada` | "O Assinador não atende este endereço." | A página não está num endereço atendido (os sistemas da Confidata em `*.confidata.app` e o uShield em `ushield.app`), ou a extensão que chamou não é a do Assinador. Com `etapa aud`, o bilhete foi emitido para outro endereço que não o da página; com `etapa padrao`, para um endereço que aquele emissor não pode usar: nos dois, o sistema que emitiu e a página que pede não combinam. | Abrir o sistema pelo endereço oficial. Se for o endereço oficial e o erro continuar, ou com `etapa aud` ou `etapa padrao`, é da engenharia. | A pessoa; a engenharia |
| `certificado-nao-encontrado` | "O certificado escolhido não está mais disponível. Conecte o token e escolha de novo." | O token ou o cartão foi tirado; o certificado venceu, ainda não vale ou não serve para assinar. No Windows, o certificado continua no repositório e o cartão dele não está na leitora (o detalhe diz "o cartão não está na leitora"). | Conectar o token e escolher de novo. Certificado vencido aparece na lista e não assina: o diagnóstico diz "vencido" na linha dele. | A pessoa |
| `chave-ausente` | "O certificado escolhido não está mais disponível. Conecte o token e escolha de novo." | O dispositivo não tem a chave do certificado, ou tem uma chave que não é a dele (no Windows, `0x80090015`, `NTE_BAD_PUBLIC_KEY`, na etapa "abrir a chave"). No Windows há duas explicações possíveis **(não medido)**: o certificado de um cartão antigo que ficou no repositório depois da renovação, ou o programa do fabricante que não sabe conferir a chave com o certificado, com o certificado CERTO e o cartão na leitora. | Tirar e recolocar o cartão e procurar os certificados de novo; com dois certificados do mesmo titular na lista, escolher o de validade mais recente. Se repetir, o diagnóstico e o código do detalhe vão para a engenharia. Nunca mande apagar certificado do repositório do Windows: apagar um certificado instalado no Windows pode apagar a chave junto, sem volta, e o que parece velho pode ser o válido. | O suporte; a engenharia |
| `algoritmo-nao-suportado` | "Este certificado usa um algoritmo que o Assinador ainda não suporta." | A chave não é RSA (só RSA na versão 1). No Linux, o programa do cartão recusou o mecanismo ou o uso da chave (`CKR_MECHANISM_INVALID`, `CKR_MECHANISM_PARAM_INVALID`, `CKR_KEY_TYPE_INCONSISTENT` ou `CKR_KEY_FUNCTION_NOT_PERMITTED` no detalhe). No Windows, o provedor do cartão recusou o SHA-256 (`NTE_BAD_ALGID` ou `NTE_NOT_SUPPORTED`; o detalhe diz "o provedor não assina SHA-256"). | Com a recusa do SHA-256 no Windows, atualizar o programa do fabricante do cartão **(não medido)**. Nos outros casos, não há contorno hoje: o diagnóstico e o detalhe vão para a engenharia. | O suporte; a engenharia |
| `protocolo` | "O Assinador respondeu de um jeito inesperado. Tente de novo; se continuar, copie o diagnóstico para o suporte." | A extensão e o programa não se entenderam (versões desencontradas) ou uma mensagem veio fora da forma. | Atualizar a extensão e o programa e tentar de novo; se repetir, o diagnóstico vai para a engenharia. | O suporte; a engenharia |
| `interno` | "O Assinador respondeu de um jeito inesperado. Tente de novo; se continuar, copie o diagnóstico para o suporte." | Falha interna: a janela da extensão não abriu, a conexão com a extensão caiu, ou a assinatura que o dispositivo devolveu não confere com o certificado (o programa a recusa antes de ela sair). | Tentar de novo; se repetir, o diagnóstico e o detalhe vão para a engenharia. "A assinatura do dispositivo não confere com o certificado" é defeito do cartão ou do programa do fabricante. | A engenharia |

## O que a página diz sem código

A biblioteca da página (`@confidata/icp-brasil`) também recusa sozinha, antes de falar com a extensão
ou depois de ouvir a resposta. Essas frases não trazem código do protocolo, e o detalhe é o que está
na segunda coluna.

| O que a pessoa lê | Detalhe | Causa provável | O que fazer | Quem resolve |
|---|---|---|---|---|
| "Para assinar com token ou cartão, instale o Assinador (uma vez neste computador)." | `extensao-ausente` | A extensão não respondeu em 1,5 s: não está instalada, está desativada, ou a aba já estava aberta antes da instalação (a extensão não entra nas abas abertas antes dela, e a aba precisa ser recarregada). Também a página fora dos endereços que a extensão atende. | Instalar a extensão pela tela de instalação, ou ativá-la na página de extensões, e recarregar a página. | A pessoa |
| "Há uma versão nova do Assinador. Atualize para continuar." (Versão abaixo da mínima) | `nativo X.Y.Z` ou `extensao X.Y.Z`: a peça e a versão que ela tem | A peça está abaixo da versão mínima que o sistema exige. | `nativo`: instalar a versão nova do programa pela tela de instalação. `extensao`: a loja atualiza a extensão sozinha, e o navegador confere de tempos em tempos; se a pessoa não pode esperar, remove a extensão e instala de novo pela loja (os endereços permitidos se perdem e são pedidos de novo). | A pessoa |
| "O Assinador não respondeu a tempo. Confira se o cartão ou o token está conectado e tente de novo." | `sem-resposta` | A extensão não respondeu nada no prazo da página (30 s para listar, 240 s para assinar). Como a extensão responde `tempo-esgotado` antes desse prazo, o silêncio quase sempre é a extensão que foi atualizada, desativada ou removida com a página aberta. | Recarregar a página e tentar de novo. | A pessoa |
| "Sessão de assinatura sem bilhete." | `sem-bilhete` | O sistema pediu a assinatura sem o bilhete: defeito de integração. | Recarregar a página; se repetir, é da engenharia. | A engenharia |
| "Não foi possível ler os certificados nesta página. Recarregue a página e tente de novo." | `leitura: ...` | A página não conseguiu ler os certificados que a extensão mandou: o código de leitura não carregou (por exemplo, depois de um deploy com a página aberta) ou o navegador falhou na conta. | Recarregar a página; se repetir, o detalhe vai para a engenharia. | A pessoa; a engenharia |

Os navegadores mínimos são o Chrome e o Edge 116 e o Firefox 140: abaixo deles, a loja não instala nem
atualiza a extensão, e a pessoa vê a frase da extensão que falta.

## Os avisos do diagnóstico

### Em qualquer sistema

| Aviso | O que fazer |
|---|---|
| "Nenhuma leitora de cartão foi encontrada. Confira o cabo USB da leitora ou do token." | Conferir o cabo, trocar de porta USB, e gerar o diagnóstico de novo. |
| "Nenhuma leitora tem cartão. Coloque o cartão na leitora (ou conecte o token)." | Colocar o cartão, com o chip para o lado certo, e gerar de novo. |
| "O cartão na leitora ... não responde. Tire o cartão e coloque de novo." | Recolocar o cartão; se repetir com a mesma leitora, testar outra leitora (contato sujo ou cartão danificado). |
| "O cartão na leitora ... não foi lido por nenhum programa de cartão instalado. Ele precisa do programa do fabricante (por exemplo, o SafeSign ou o SafeNet)." | Descobrir o fabricante do cartão (o ATR do diagnóstico ajuda) e instalar o programa dele. |
| "O programa do cartão ... falhou (...)." | No Linux, o programa do fabricante não carregou, caiu ou passou de 20 s na listagem ("não respondeu no prazo"): reinstalar, e mandar o diagnóstico à engenharia se repetir. No Windows, "Windows (repositório do usuário)" com "abrir o repositório: 0x..." é o repositório de certificados que não abriu: veja o aviso do repositório abaixo. |
| "O certificado de ... venceu em ...: ele aparece na lista, mas não assina." | Renovar o certificado com a certificadora. |
| "O PC/SC não respondeu como devia (...)." | Mandar o diagnóstico à engenharia com o código entre parênteses. |

### Linux

| Aviso | O que fazer |
|---|---|
| "A biblioteca do PC/SC não está instalada, e sem ela nenhuma leitora aparece. Instale o pacote pcscd (no Fedora, pcsc-lite)." | `sudo apt install pcscd` (Debian, Ubuntu) ou `sudo dnf install pcsc-lite` (Fedora). |
| "O serviço pcscd não está rodando, e sem ele nenhuma leitora aparece. Para iniciar: sudo systemctl start pcscd." | `sudo systemctl start pcscd`; se não subir, `sudo systemctl enable --now pcscd.socket`. |
| "Nenhum programa de cartão (módulo PKCS#11) foi encontrado neste computador. Instale o do fabricante do cartão ou do token." | Instalar o programa do fabricante. O Assinador acha sozinho os do catálogo medido e os registrados no p11-kit; outro módulo entra, um caminho por linha, em `~/.config/confidata-assinador/modulos`. |
| "O cartão na leitora ... usa o ..., que não está instalado. Instale o ..." | Instalar o programa do fabricante indicado (o ATR do cartão está no catálogo medido). |
| "O ... está instalado, mas não achou certificado no cartão da leitora ..." | O programa do fabricante carregou e não leu certificado no cartão. Pode ser cartão sem certificado emitido, ou certificado apagado; antes de mandar à certificadora, conferir no diagnóstico se o cartão da leitora é mesmo o que a pessoa usa para assinar. |

### Windows

| Aviso | O que fazer |
|---|---|
| "O componente de cartão inteligente do Windows (winscard.dll) não abriu, e sem ele nenhuma leitora aparece." | Windows danificado ou restrito por política: é da TI da pessoa. |
| "Nenhuma leitora de cartão foi encontrada, ou o serviço Cartão Inteligente do Windows está parado. Confira o cabo USB da leitora ou do token." | Conferir o cabo e trocar de porta. Sem leitora conectada, o Windows pode manter o serviço Cartão Inteligente parado, e por isso as duas causas vêm juntas **(não medido)**. Com a leitora conectada e o aviso repetindo, abrir Serviços (services.msc), Cartão Inteligente, Iniciar. |
| "O serviço Propagação de Certificados do Windows está parado, e com ele parado o certificado do cartão não entra na lista. Para iniciar: abra Serviços (services.msc), Propagação de Certificados, Iniciar." | Iniciar o serviço, tirar o cartão e colocar de novo (o serviço copia o certificado quando o cartão entra), e gerar o diagnóstico de novo. Se a TI da empresa desligou o serviço por política, é com ela. É uma causa provável de "o cartão está na leitora e o certificado não aparece" **(não medido)**. |
| "O cartão na leitora ... usa o .... Se o certificado não aparece na lista, instale o ... para Windows." | Instalar o programa do fabricante para Windows. |
| "O programa do cartão Windows (repositório do usuário) falhou (abrir o repositório: 0x...)." | O repositório de certificados do usuário não abriu, e a lista pode estar incompleta: fechar a sessão do Windows e entrar de novo; se repetir, é da TI, com o código entre parênteses. Não é o programa do fabricante, e reinstalá-lo não resolve. |

### Outros sistemas

| Aviso | O que fazer |
|---|---|
| "O Assinador ainda não lê as leitoras de cartão neste sistema." | O sistema não é Linux nem Windows (o macOS ainda não é suportado). Não há contorno hoje. |

### As linhas de certificado

Cada certificado aparece numa linha "Certificado:", com a situação dele. Os `vencido` e `ainda não
vale` aparecem também na lista da página e não assinam (a recusa é `certificado-nao-encontrado`, com o
detalhe "certificado vencido" ou "certificado ainda não vale"); os `sem uso de assinatura`, `chave que
não é RSA` e `ilegível` ficam só no diagnóstico, e a página nem os mostra.

| Situação | O que quer dizer | O que fazer |
|---|---|---|
| `válido` | Pode assinar. | Se ele não aparece na página, o problema é outro: veja os avisos. |
| `vencido` | Passou da validade. Aparece na lista da página e não assina. | Renovar com a certificadora. |
| `ainda não vale` | A validade ainda não começou, ou o relógio do computador está atrasado. | Conferir a data do computador; se estiver certa, esperar o início da validade. |
| `sem uso de assinatura` | O certificado não é para assinar (é de cifra, por exemplo). | Usar o outro certificado do cartão, se houver; senão, é com a certificadora. |
| `chave que não é RSA` | O Assinador só assina com RSA na versão 1. | Não há contorno hoje. |
| `ilegível` | O programa não conseguiu ler o certificado. | Mandar o diagnóstico à engenharia. |

## Temas que voltam

### O programa do fabricante (middleware)

O Assinador não fala com o chip: quem lê o cartão é o programa do fabricante (SafeSign, SafeNet e
outros). Sem ele, o cartão aparece no diagnóstico e o certificado não. No Linux, o Assinador procura
os programas do catálogo medido, os registrados no p11-kit e os que a pessoa lista em
`~/.config/confidata-assinador/modulos`; o que falha ou passa de 20 s na listagem é deixado de lado,
e os certificados dele não aparecem. No Windows, o programa do fabricante se registra no próprio
Windows, e o certificado do cartão entra no repositório pessoal pelo serviço de Propagação de
Certificados.

### Windows: instalação por usuário e por máquina

O programa tem dois instaladores no Windows: o por usuário (sem administrador, o que a tela de
instalação oferece) e o por máquina (para a TI distribuir por GPO ou Intune). Dois casos voltam:

- **Empresa que desligou o programa por usuário.** Com a política `NativeMessagingUserLevelHosts` do
  Chrome ou do Edge desligada, o navegador não abre programa instalado por usuário, e a extensão diz
  que falta o programa com ele instalado. Só o instalador por máquina funciona ali, e quem o instala é
  a TI. É o comportamento documentado da política **(não medido)**.
- **Os dois instalados na mesma máquina.** O Chrome e o Edge leem a instalação do usuário antes da
  da máquina, e um instalador não remove o outro. Se o por usuário for um pacote de desenvolvimento, o
  navegador segue abrindo ele, que não aceita a extensão da loja (`nativo-ausente` com `falhou`).
  Remover a instalação do usuário (Configurações do Windows, Aplicativos) resolve. Os pacotes de
  desenvolvimento nunca são oferecidos em página pública, então o caso é de quem testa.

### PIN e PUK

- O PIN é pedido pela janela de confirmação da extensão (Linux) ou pelo diálogo de PIN do provedor do
  cartão (Windows), que pode ser o do próprio Windows ou um do programa do fabricante, conforme o
  cartão **(não medido)**. O Assinador nunca guarda o PIN e nunca o repete sozinho depois de um erro.
- No Windows, há programa de fabricante que pede o PIN de novo no próprio diálogo depois de um erro,
  sem devolver a recusa ao Assinador: aí quem conta as tentativas é o diálogo do fabricante
  **(não medido)**.
- Cartão bloqueado se desbloqueia com o PUK, pelo programa do fabricante (a opção de desbloquear o
  PIN), com o PUK que a certificadora entregou na emissão. Sem o PUK, só a certificadora.
- O PUK também tem tentativas contadas. Esgotadas, o cartão em geral fica inutilizável, e só a
  certificadora emite outro. Na dúvida sobre o PUK, a pessoa para e procura a certificadora antes de
  tentar.
- PIN VENCIDO (Linux, `CKR_PIN_EXPIRED` no detalhe do `token-bloqueado`) não é bloqueio: troca-se o
  PIN pelo programa do fabricante, sem o PUK.

### O relógio

O pedido de assinatura vale da emissão menos 15 minutos até o vencimento mais 15 minutos. Relógio
fora disso dá `relogio`. O acerto automático de data e hora resolve; com dual boot (Linux e Windows
na mesma máquina), um dos dois costuma gravar a hora local como se fosse a universal.

### Navegador em Snap (Linux)

O Firefox e o Chromium instalados como Snap rodam confinados, e ainda não foi medido se eles abrem o
programa (item (c) de `docs/medicoes/F0.md`). Se a extensão diz que falta o programa com ele
instalado, o contorno é o navegador do pacote `.deb` (o Google Chrome, ou o Firefox do repositório
da Mozilla).

### A permissão por endereço

A extensão só atende o endereço que a pessoa permitiu, uma vez por endereço, na janela de permissão.
A lista dos permitidos fica nas opções da extensão, com "Remover" em cada um; a recusa não fica
guardada, e o endereço recusado pede de novo no próximo uso. Endereço que insiste em abrir janela
recusada entra em embargo (10 minutos na permissão, 2 na confirmação), sem janela nenhuma: é proteção
contra página que força a pessoa a ceder, e passa sozinho, ou quando o navegador é fechado e aberto.
