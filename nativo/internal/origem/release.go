//go:build !dev

package origem

// No build de release, `localhost` nunca é origem aceita sem bilhete, e só as extensões publicadas
// chamam o programa.
const localhostSemBilhete = false

// extensoesChromeDev são os IDs da extensão de DESENVOLVIMENTO no Chrome e no Edge. Vazio no build
// de release.
var extensoesChromeDev []string
