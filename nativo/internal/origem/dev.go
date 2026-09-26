//go:build dev

package origem

// No build de desenvolvimento, `localhost` também pode listar e diagnosticar sem bilhete. Bilhete
// para `localhost` continua exigindo chave de ambiente `dev` (regra de `Aceita`).
const localhostSemBilhete = true

// extensoesChromeDev são os IDs da extensão de DESENVOLVIMENTO no Chrome e no Edge (a mesma
// extensão descompactada tem o mesmo ID nos dois), derivados da chave pública de
// `protocolo/extensao-dev.json`; `dev_test.go` confere a derivação. Provisório: a F3 troca a chave
// pela do rascunho do item na loja.
var extensoesChromeDev = []string{"jmogljhnfdnhhoapijclifkpjfbhhppf"}
