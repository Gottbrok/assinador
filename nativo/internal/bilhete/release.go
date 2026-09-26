//go:build !dev

package bilhete

// No build de release não existe chave de desenvolvimento: nem o caminho do arquivo local entra no
// binário (a catraca de `release_test.go` confere os bytes do executável).
func chavesDeDesenvolvimento([]Chave) ([]Chave, []string) {
	return nil, nil
}
