# Assinador

Componente para assinar com certificado digital A3 (token ou cartão ICP-Brasil) a partir do
navegador, sem que a chave saia do dispositivo da pessoa.

É formado por um programa nativo (Windows, Linux e, depois, macOS) e uma extensão para Chrome, Edge e
Firefox. A página nunca fala com o cartão: ela pede à extensão, que pede ao programa, que só assina um
resumo acompanhado de um bilhete emitido pelo servidor que preparou o documento. A janela de
confirmação da extensão mostra à pessoa o documento verdadeiro antes de pedir o PIN.

O programa não abre porta de rede, não acessa a internet e não guarda o PIN.

Em construção. Licença Apache-2.0.
