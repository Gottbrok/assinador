package bilhete

//go:generate go run ./gerar -entrada ../../../protocolo/chaves-publicas.json -saida chaves.go

// jwkPinada é uma entrada do `chaves.go` gerado: as partes públicas de uma JWK P-256.
type jwkPinada struct {
	Kid      string
	Emissor  string
	Ambiente string
	X        string
	Y        string
}

// ChavesDoPrograma são as chaves em que ESTE programa acredita: as de produção pinadas em
// `chaves.go` (gerado de `protocolo/chaves-publicas.json`) e, só no build `dev`, as de
// desenvolvimento lidas do arquivo local. O segundo retorno são avisos para o registro de erros
// do programa (arquivo de desenvolvimento ilegível, entrada recusada), nunca para a página.
//
// Entrada pinada que não forma uma chave válida fica de fora: o que não é chave não confere
// bilhete nenhum (falha fechada). O teste do gerador impede que isso chegue a um release.
func ChavesDoPrograma() ([]Chave, []string) {
	chaves := make([]Chave, 0, len(chavesPinadas))
	var avisos []string
	for _, p := range chavesPinadas {
		c, err := ChaveDeJwk(p.Kid, p.Emissor, p.Ambiente, p.X, p.Y)
		if err != nil {
			avisos = append(avisos, "chave pinada "+p.Kid+" recusada: "+err.Error())
			continue
		}
		chaves = append(chaves, c)
	}
	dev, avisosDev := chavesDeDesenvolvimento(chaves)
	return append(chaves, dev...), append(avisos, avisosDev...)
}
