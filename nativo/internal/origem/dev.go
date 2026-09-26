//go:build dev

package origem

// No build de desenvolvimento, `localhost` também pode listar e diagnosticar sem bilhete. Bilhete
// para `localhost` continua exigindo chave de ambiente `dev` (regra de `Aceita`).
const localhostSemBilhete = true

// extensoesChromeDev são os IDs da extensão de DESENVOLVIMENTO no Chrome e no Edge, derivados da
// `key` do manifesto de desenvolvimento. Entram com a F3, que cria a extensão.
var extensoesChromeDev []string
