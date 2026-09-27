# Suporte do Assinador

Para quem atende o chamado de quem não consegue assinar com token ou cartão. Cada código de erro do
protocolo e cada aviso do diagnóstico, com a causa provável e o que fazer. A coluna "Quem resolve" diz
o que a própria pessoa resolve (é o que o artigo da Central de Ajuda explica) e o que é do suporte ou
da engenharia.

O que está aqui é o que o código faz hoje. O que ainda não foi medido com cartão real está marcado
assim, e a medição mora em `docs/medicoes/`.

## Nunca peça

- O PIN nem o PUK, por nenhum canal. O Assinador não os guarda e o suporte não precisa deles.
- O arquivo do certificado, a senha dele, ou foto do cartão.
- O CPF. O diagnóstico não tem CPF (o nome do titular sai com os dígitos trocados por `*`); se a
  pessoa mandar uma captura de tela com o CPF, descarte a captura.
- Que a pessoa apague certificado, do repositório do Windows ou do token. Apagar pode levar a chave
  junto, sem volta, e o certificado que parece velho pode ser o válido: o caso vai para a engenharia.

## O primeiro pedido: o diagnóstico

O diagnóstico é o relatório do Assinador para o suporte: o sistema, as leitoras e o cartão em cada
uma, os programas de cartão com o estado de cada um, os certificados (com o nome mascarado) e os
avisos em frase. Três jeitos de conseguir, do mais fácil para o mais difícil:

1. **Pela extensão:** nas opções da extensão, "Gerar diagnóstico" e "Copiar diagnóstico". As opções
   abrem pela página de extensões do navegador: no Chrome e no Edge, "Extensões", o Assinador,
   "Detalhes", "Opções da extensão"; no Firefox, "Complementos e temas", o Assinador, "Opções".
   Funciona mesmo sem nenhum endereço autorizado.
2. **Pelo terminal, no Linux:** `/usr/lib/confidata-assinador/assinador diagnostico` (com `--json`,
   o mesmo relatório em JSON).
3. **Pelo terminal, no Windows:** `assinador.exe diagnostico`, na pasta onde o instalador pôs o
   programa.

Se a extensão diz que não consegue gerar o diagnóstico (`nativo-ausente` entre parênteses), o
programa não está instalado ou o navegador não o acha: veja `nativo-ausente` abaixo.

## Os códigos de erro

A página mostra a frase; o código e o detalhe técnico vêm junto, no erro que a página recebe. O
detalhe é texto para o suporte (no Windows, com a etapa e o código do Windows em hexadecimal) e nunca
tem PIN nem CPF.

| Código | O que a pessoa lê | Causa provável | O que fazer | Quem resolve |
|---|---|---|---|---|
| `cancelado` | "Assinatura cancelada no Assinador." | A pessoa cancelou ou fechou a janela de confirmação, fechou o diálogo de PIN (no Windows, o diálogo é do Windows), ou saiu da página no meio. Com o detalhe `embargo`, a página abriu janela de confirmação recusada três vezes em 10 minutos, e a extensão recusa por 2 minutos sem abrir janela. | Assinar de novo. Com `embargo`, esperar 2 minutos. | A pessoa |
| `permissao-negada` | "Você não permitiu que este endereço use o Assinador. A permissão se muda nas opções da extensão." | A pessoa negou a janela de permissão do endereço. Com `embargo`, três janelas de permissão recusadas em 10 minutos, e o endereço fica 10 minutos sem janela. No Firefox, também o diagnóstico pedido pela página com o envio desligado nas opções. | Nas opções da extensão, permitir o endereço (ou remover a recusa e tentar de novo). Com `embargo`, esperar 10 minutos. | A pessoa |
| `pin-incorreto` | "PIN incorreto." Com "Restam poucas tentativas" ou "Resta UMA tentativa", quando o cartão informa. | O PIN digitado está errado. O Assinador nunca repete o PIN sozinho: cada erro é uma tentativa gasta. No Windows o Windows não informa quantas restam. | Conferir o PIN com calma antes de tentar de novo. Na última tentativa, parar e procurar o PUK antes (veja "PIN e PUK"). | A pessoa |
| `token-bloqueado` | "O cartão bloqueou por excesso de tentativas. O desbloqueio é com o PUK, pela sua certificadora." | O PIN foi errado vezes demais e o cartão bloqueou (ou já chegou bloqueado). | Desbloquear com o PUK (veja "PIN e PUK"). O Assinador não desbloqueia cartão. | A pessoa, com a certificadora |
| `nativo-ausente` | "A extensão está instalada. Falta o programa que lê o cartão." | O programa não está instalado, ou o navegador não acha o manifesto dele. No Linux, o navegador em Snap pode não conseguir abrir o programa (veja "Navegador em Snap", ainda não medido). | Instalar o programa pela tela de instalação e reabrir o navegador. No Linux com Snap, usar o navegador do pacote `.deb`. | A pessoa; o suporte se persistir |
| `nativo-desatualizado` | "Há uma versão nova do Assinador. Atualize para continuar." | O programa instalado é mais velho que a versão mínima que o sistema exige. | Instalar a versão nova pela tela de instalação. | A pessoa |
| `tempo-esgotado` | "O Assinador não recebeu resposta a tempo. Tente de novo." | O programa do cartão travou (no Linux, cada módulo tem 20 s para listar e 90 s para assinar), o dispositivo ficou ocupado o prazo inteiro, ou a janela de permissão ou de confirmação não foi respondida a tempo. | Desconectar e reconectar o token ou o cartão, fechar outro programa que esteja usando o cartão, e tentar de novo. Se repetir, o diagnóstico mostra qual programa de cartão falhou. | A pessoa; o suporte se repetir |
| `modulo-falhou` | "O programa do cartão ou do token falhou. Reconecte o dispositivo e tente de novo." | No Linux, o programa do fabricante (módulo PKCS#11) não carregou ou caiu. No Windows, o provedor do fabricante recusou: o detalhe traz o código. `0x80090019` (`NTE_KEYSET_NOT_DEF`) é o provedor do fabricante que não está instalado; `0x80090010` e `0x80070005` são acesso negado; `0x80004005` é recusa sem motivo. Também é o programa do Assinador que saiu no meio. | Reconectar e tentar de novo. Se repetir: o diagnóstico, e reinstalar o programa do fabricante do cartão. | O suporte |
| `ocupado` | "O Assinador está ocupado com outra operação. Conclua ou cancele a janela aberta e tente de novo." | Outra assinatura em curso (outra aba, outra janela de confirmação aberta), ou a fila do dispositivo cheia. | Concluir ou fechar a outra janela e tentar de novo. | A pessoa |
| `bilhete-invalido` | "A sessão de assinatura expirou. Procure os certificados de novo." | O pedido de assinatura que o sistema preparou não passou na conferência do programa: chave do emissor que o programa não conhece (programa velho para uma chave nova), bilhete malformado ou assinatura inválida. | Recarregar a página e procurar os certificados de novo. Se repetir, atualizar o programa; se ainda repetir, é da engenharia (o programa e o sistema não se entendem). | O suporte; a engenharia |
| `bilhete-expirado` | "A sessão de assinatura expirou. Procure os certificados de novo." | Hoje nenhuma das pontas o produz: está no vocabulário do protocolo, e o bilhete fora do prazo sai como `relogio`. Se aparecer, veio de uma versão nova de alguma ponta. | Recarregar a página e procurar os certificados de novo; se repetir, é da engenharia. | A engenharia |
| `relogio` | "O relógio deste computador está errado. Acerte a data e a hora e tente de novo." | A hora do computador está fora da janela do pedido (a emissão menos 15 minutos até o vencimento mais 15 minutos). Quase sempre é relógio errado: dual boot, bateria do computador fraca, ou fuso trocado com a hora acertada à mão (a hora parece certa e o horário universal está errado). | Ligar o acerto automático de data e hora e conferir o fuso horário; depois recarregar a página. | A pessoa |
| `digest-divergente` | "A sessão de assinatura expirou. Procure os certificados de novo." | A página pediu para assinar um conteúdo diferente do que o sistema autorizou. | Recarregar e tentar de novo. Se repetir, é da engenharia, e é sinal de segurança (página alterada no caminho): registrar o endereço e a hora. | A engenharia |
| `certificado-divergente` | "A sessão de assinatura expirou. Procure os certificados de novo." | O certificado escolhido não é o que o sistema preparou (a pessoa trocou de token ou de certificado entre a busca e a assinatura). | Procurar os certificados de novo e escolher o certificado de novo. | A pessoa |
| `origem-recusada` | "O Assinador não atende este endereço." | A página não está num endereço atendido (os sistemas da Confidata em `*.confidata.app` e o uShield em `ushield.app`), ou a extensão que chamou não é a do Assinador. | Abrir o sistema pelo endereço oficial. Se for o endereço oficial e o erro continuar, é da engenharia. | A pessoa; a engenharia |
| `certificado-nao-encontrado` | "O certificado escolhido não está mais disponível. Conecte o token e escolha de novo." | O token ou o cartão foi tirado; o certificado venceu, ainda não vale ou não serve para assinar. No Windows, o certificado continua no repositório e o cartão dele não está na leitora (o detalhe diz "o cartão não está na leitora"). | Conectar o token e escolher de novo. Certificado vencido aparece na lista e não assina: o diagnóstico diz "venceu em". | A pessoa |
| `chave-ausente` | "O certificado escolhido não está mais disponível. Conecte o token e escolha de novo." | O dispositivo não tem a chave do certificado, ou tem uma chave que não é a dele (no Windows, `0x80090015`, `NTE_BAD_PUBLIC_KEY`, na etapa "abrir a chave"). No Windows ainda não foi medido com cartão real, e há duas explicações possíveis: o certificado de um cartão antigo que ficou no repositório depois da renovação, ou o programa do fabricante que não sabe conferir a chave com o certificado, com o certificado CERTO e o cartão na leitora. | Tirar e recolocar o cartão e procurar os certificados de novo; com dois certificados do mesmo titular na lista, escolher o de validade mais recente. Se repetir, o diagnóstico e o código do detalhe vão para a engenharia. Nunca mande apagar certificado do repositório do Windows: apagar um certificado instalado no Windows pode apagar a chave junto, sem volta, e o que parece velho pode ser o válido. | O suporte; a engenharia |
| `algoritmo-nao-suportado` | "Este certificado usa um algoritmo que o Assinador ainda não suporta." | A chave não é RSA (só RSA na versão 1), ou, no Windows, o provedor antigo do fabricante não assina SHA-256 (o detalhe diz "o provedor não assina SHA-256"). | Com o provedor sem SHA-256, atualizar o programa do fabricante do cartão. Com chave que não é RSA, não há contorno hoje: é da engenharia. | O suporte; a engenharia |
| `protocolo` | "O Assinador respondeu de um jeito inesperado. Tente de novo; se continuar, copie o diagnóstico para o suporte." | A extensão e o programa não se entenderam (versões desencontradas) ou uma mensagem veio fora da forma. | Atualizar a extensão e o programa e tentar de novo; se repetir, o diagnóstico vai para a engenharia. | O suporte; a engenharia |
| `interno` | "O Assinador respondeu de um jeito inesperado. Tente de novo; se continuar, copie o diagnóstico para o suporte." | Falha interna: a janela da extensão não abriu, a conexão com a extensão caiu, ou a assinatura que o dispositivo devolveu não confere com o certificado (o programa a recusa antes de ela sair). | Tentar de novo; se repetir, o diagnóstico e o detalhe vão para a engenharia. "A assinatura do dispositivo não confere com o certificado" é defeito do cartão ou do programa do fabricante. | A engenharia |

## Os avisos do diagnóstico

### Em qualquer sistema

| Aviso | O que fazer |
|---|---|
| "Nenhuma leitora de cartão foi encontrada. Confira o cabo USB da leitora ou do token." | Conferir o cabo, trocar de porta USB, e gerar o diagnóstico de novo. |
| "Nenhuma leitora tem cartão. Coloque o cartão na leitora (ou conecte o token)." | Colocar o cartão, com o chip para o lado certo, e gerar de novo. |
| "O cartão na leitora ... não responde. Tire o cartão e coloque de novo." | Recolocar o cartão; se repetir com a mesma leitora, testar outra leitora (contato sujo ou cartão danificado). |
| "O cartão na leitora ... usa o ..., que não está instalado. Instale o ..." | Instalar o programa do fabricante indicado (o ATR do cartão está no catálogo medido). |
| "O ... está instalado, mas não achou certificado no cartão da leitora ..." | O programa do fabricante lê o cartão e o cartão não tem certificado: cartão novo sem emissão, ou certificado apagado. É com a certificadora. |
| "O cartão na leitora ... não foi lido por nenhum programa de cartão instalado. Ele precisa do programa do fabricante (por exemplo, o SafeSign ou o SafeNet)." | Descobrir o fabricante do cartão (o ATR do diagnóstico ajuda) e instalar o programa dele. |
| "O programa do cartão ... falhou (...)." | O programa do fabricante não carregou ou caiu: reinstalar, e mandar o diagnóstico à engenharia se repetir. |
| "Um certificado com chave que não é RSA foi ignorado: o Assinador ainda não assina com ela." | Não há contorno hoje (só RSA na versão 1). |
| "Um certificado do dispositivo não pôde ser lido e foi ignorado." | Certificado corrompido no dispositivo: é com a certificadora. |
| "O certificado de ... venceu em ...: ele aparece na lista, mas não assina." | Renovar o certificado com a certificadora. |
| "O PC/SC não respondeu como devia (...)." | Mandar o diagnóstico à engenharia com o código entre parênteses. |

### Linux

| Aviso | O que fazer |
|---|---|
| "A biblioteca do PC/SC não está instalada, e sem ela nenhuma leitora aparece. Instale o pacote pcscd (no Fedora, pcsc-lite)." | `sudo apt install pcscd` (Debian, Ubuntu) ou `sudo dnf install pcsc-lite` (Fedora). |
| "O serviço pcscd não está rodando, e sem ele nenhuma leitora aparece. Para iniciar: sudo systemctl start pcscd." | `sudo systemctl start pcscd`; se não subir, `sudo systemctl enable --now pcscd.socket`. |
| "Nenhum programa de cartão (módulo PKCS#11) foi encontrado neste computador. Instale o do fabricante do cartão ou do token." | Instalar o programa do fabricante. O Assinador acha sozinho os do catálogo medido e os registrados no p11-kit; outro módulo entra, um caminho por linha, em `~/.config/confidata-assinador/modulos`. |

### Windows

| Aviso | O que fazer |
|---|---|
| "O componente de cartão inteligente do Windows (winscard.dll) não abriu, e sem ele nenhuma leitora aparece." | Windows danificado ou restrito por política: é da TI da pessoa. |
| "Nenhuma leitora de cartão foi encontrada, ou o serviço Cartão Inteligente do Windows está parado. Confira o cabo USB da leitora ou do token." | O Windows só mantém o serviço Cartão Inteligente rodando com uma leitora conectada: conferir o cabo e trocar de porta. |
| "O serviço Propagação de Certificados do Windows está parado, e com ele parado o certificado do cartão não entra na lista. Para iniciar: abra Serviços (services.msc), Propagação de Certificados, Iniciar." | É o caso mais comum de "o cartão está na leitora e o certificado não aparece". Iniciar o serviço; se a TI da empresa o desligou por política, é com ela. |
| "O cartão na leitora ... usa o .... Se o certificado não aparece na lista, instale o ... para Windows." | Instalar o programa do fabricante para Windows. |
| "O repositório de certificados do Windows não abriu, e a lista pode estar incompleta." | Perfil do Windows com problema: fechar a sessão do Windows e entrar de novo; se repetir, é da TI. |

## Temas que voltam

### O programa do fabricante (middleware)

O Assinador não fala com o chip: quem lê o cartão é o programa do fabricante (SafeSign, SafeNet e
outros). Sem ele, o cartão aparece no diagnóstico e o certificado não. No Linux, o Assinador procura
os programas do catálogo medido, os registrados no p11-kit e os que a pessoa lista em
`~/.config/confidata-assinador/modulos`. No Windows, o programa do fabricante se registra no próprio
Windows, e o certificado do cartão entra no repositório pessoal pelo serviço de Propagação de
Certificados.

### PIN e PUK

- O PIN é pedido pela janela de confirmação da extensão (Linux) ou pelo diálogo do programa do
  fabricante (Windows). O Assinador nunca guarda o PIN e nunca o repete sozinho depois de um erro.
- No Windows, há programa de fabricante que pede o PIN de novo no próprio diálogo depois de um erro,
  sem devolver a recusa ao Assinador: aí quem conta as tentativas é o diálogo do fabricante (ainda não
  medido com cartão real).
- Cartão bloqueado se desbloqueia com o PUK, pelo programa do fabricante (a opção de desbloquear o
  PIN), com o PUK que a certificadora entregou na emissão. Sem o PUK, só a certificadora.

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
A lista fica nas opções da extensão, com "Remover" em cada um. Endereço que insiste em abrir janela
recusada entra em embargo por alguns minutos (10 na permissão, 2 na confirmação), sem janela nenhuma:
é proteção contra página que força a pessoa a ceder, e passa sozinho.
