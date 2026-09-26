# Segurança

O Assinador toca a chave privada do certificado digital de uma pessoa. Este arquivo diz o que ele
promete, contra o que se defende, e como relatar uma falha.

## Como relatar uma vulnerabilidade

Pelo relato privado de vulnerabilidade do GitHub, na aba **Security** deste repositório ("Report a
vulnerability"). Não abra issue pública para falha de segurança. Diga a versão do programa e da
extensão, o sistema, e o passo a passo; nunca mande PIN, chave privada ou certificado de outra
pessoa.

## O que o programa promete

- **Só assina com bilhete conferido.** O bilhete é um JWS ES256 emitido pelo servidor que preparou
  o resumo, com a origem da página, o resumo, o certificado e um prazo curto. O programa confere a
  assinatura dele contra chaves PÚBLICAS pinadas no próprio binário, antes de carregar qualquer
  biblioteca de cartão, e confere de novo no `assinar`. Um script injetado numa página nossa não
  consegue pedir a assinatura de um resumo que o nosso servidor não preparou.
- **Não abre porta de rede e não acessa a internet**, em nenhum modo. Fala só com a extensão, pela
  entrada e saída padrão, e com os módulos PKCS#11, em processos filhos.
- **Não guarda nem devolve o PIN.** O PIN nunca vira `string` no programa: é lido do pedido para
  um buffer de capacidade fixa, vai ao filho do módulo num quadro próprio, é copiado para memória
  do C só durante o `C_Login` e é zerado depois. Nunca vai a log, disco ou resposta. Um PIN errado
  encerra a operação: o programa nunca tenta de novo sozinho.
- **Isola as bibliotecas de fabricante.** Cada módulo PKCS#11 roda num processo filho, com a
  saída padrão em `/dev/null`: biblioteca que cai derruba só o filho, e biblioteca que imprime na
  tela não corrompe o canal.
- **Confere o que o cartão devolveu.** A assinatura é conferida contra a chave pública do
  certificado antes de sair: chave trocada no token não produz assinatura que alguém aceitaria.
- **Não interpreta campo ICP-Brasil nem mostra CPF.** O nome, o CPF e a empresa do certificado quem
  lê é a biblioteca, no navegador; o que o programa monta em texto sai com os dígitos mascarados.
- **Recusa o que não é do protocolo.** O pedido é lido por um leitor JSON estrito próprio: chave
  repetida, chave desconhecida, tipo errado, UTF-8 inválido e mensagem acima do teto são recusados.
- **O build de release só acredita em chave de produção.** Chave de teste (cujas privadas são
  públicas) e o carregador de chaves de desenvolvimento não entram no binário de release; um teste
  confere os bytes do executável.

## O que a extensão promete

- **Só atende quem o navegador diz que pediu.** O script de conteúdo aceita só a mensagem da própria
  janela, da origem dela, no quadro de topo; o fundo confere de novo o remetente e a origem pelo que
  o NAVEGADOR diz (nunca pelo que a mensagem declara), contra os endereços dos emissores.
- **Nada além de `ola` sem a permissão da pessoa para aquele endereço**, perguntada numa janela da
  própria extensão e guardada só neste computador. Sem ela, nenhuma página lê os certificados.
- **Nenhuma assinatura sem a janela de confirmação**, que mostra o que o PROGRAMA leu do bilhete
  conferido (o documento, a organização) e o endereço que o navegador diz. A página não alcança a
  janela. Os botões só respondem 600 ms depois de ela ficar visível e com foco, travam de novo
  quando ela perde o foco, ignoram tecla segurada, e o foco nasce no controle seguro (o PIN, ou
  "Cancelar"; nunca "Assinar"): contra clique, clique duplo ou Enter cronometrado pela página. Uma
  assinatura por vez; a página que sai, ou o programa que cai, fecha a janela.
- **Uma página não esgota o computador nem a pessoa.** O `ola` é um só em voo; os pedidos que tocam
  o cartão fazem fila (um por vez, com teto); e três janelas recusadas seguidas embargam o endereço
  por alguns minutos, sem janela nenhuma.
- **Não acessa a internet e não guarda documento, assinatura nem PIN.** O PIN digitado na janela vai
  direto ao programa. O build reprova qualquer acesso de rede no pacote, e o pacote publicado pode
  ser refeito a partir do código da versão (com o `SOURCE_DATE_EPOCH` publicado) com o mesmo
  SHA-256.

## O que está fora do alcance

- Um programa malicioso rodando no computador da pessoa, com o usuário dela, já alcança o cartão
  sem nós (e pode capturar o PIN no teclado). O Assinador não se defende de quem já controla o
  computador.
- A página decide O QUE assinar junto com o servidor dela; o programa garante que só assina o que
  o servidor emissor autorizou, para aquela origem, aquele certificado e aquele resumo, e a janela
  da extensão mostra o documento verdadeiro.
- Um script que já roda num endereço que a pessoa permitiu (por exemplo, por uma falha de XSS
  nele) vê os certificados do computador, como a própria página veria. Ele não assina: sem o
  bilhete do servidor emissor o programa recusa, e com ele a pessoa ainda vê a janela de
  confirmação.

## Chaves de bilhete

As chaves privadas de bilhete nascem dentro dos servidores emissores, nos consoles deles, e nunca
entram neste repositório. Aqui ficam só as PÚBLICAS de produção (`protocolo/chaves-publicas.json`).
Chave comprometida: publica-se uma versão do programa que pina só a nova e sobe-se a versão mínima;
os programas antigos deixam de assinar para aquele emissor até atualizarem.
