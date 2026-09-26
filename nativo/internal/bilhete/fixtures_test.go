package bilhete

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// As fixtures são as da biblioteca, byte a byte: cada arquivo tem a soma registrada em
// `protocolo/fixtures/ORIGEM.md`, e não há arquivo a mais nem a menos. Fixture editada à mão aqui
// deixaria de ser o contrato com a referência em TypeScript.
func TestFixturesSaoAsDaTag(t *testing.T) {
	raiz := filepath.Join("..", "..", "..", "protocolo", "fixtures")
	f, err := os.Open(filepath.Join(raiz, "ORIGEM.md"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	somas := map[string]string{}
	dentro := false
	s := bufio.NewScanner(f)
	for s.Scan() {
		linha := s.Text()
		if linha == "```" {
			dentro = !dentro
			continue
		}
		if !dentro {
			continue
		}
		soma, caminho, ok := strings.Cut(linha, "  ")
		if !ok || len(soma) != 64 {
			t.Fatalf("linha fora do formato do sha256sum: %q", linha)
		}
		somas[caminho] = soma
	}
	if len(somas) == 0 {
		t.Fatal("ORIGEM.md sem somas")
	}
	var achados []string
	err = filepath.WalkDir(filepath.Join(raiz, "bilhete"), func(caminho string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(raiz, caminho)
		rel = filepath.ToSlash(rel)
		achados = append(achados, rel)
		bruto, err := os.ReadFile(caminho)
		if err != nil {
			return err
		}
		h := sha256.Sum256(bruto)
		quer, ok := somas[rel]
		if !ok {
			t.Errorf("%s não está no ORIGEM.md", rel)
		} else if hex.EncodeToString(h[:]) != quer {
			t.Errorf("%s mudou depois da cópia", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for caminho := range somas {
		if !slices.Contains(achados, caminho) {
			t.Errorf("%s está no ORIGEM.md e sumiu da pasta", caminho)
		}
	}
}
