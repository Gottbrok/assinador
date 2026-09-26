//go:build dev

package origem

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// O ID de desenvolvimento é o que o Chrome deriva da chave pública de `protocolo/extensao-dev.json`
// (os 16 primeiros bytes do SHA-256 da SubjectPublicKeyInfo, com os dígitos hexadecimais 0 a f
// trocados por a a p).
func TestIdDeDesenvolvimentoVemDaChave(t *testing.T) {
	bruto, err := os.ReadFile(filepath.Join("..", "..", "..", "protocolo", "extensao-dev.json"))
	if err != nil {
		t.Fatal(err)
	}
	var arquivo struct {
		Chave    string `json:"chave"`
		IDChrome string `json:"idChrome"`
	}
	if err := json.Unmarshal(bruto, &arquivo); err != nil {
		t.Fatal(err)
	}
	der, err := base64.StdEncoding.DecodeString(arquivo.Chave)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(der)
	id := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return 'a' + (r - '0')
		}
		return 'k' + (r - 'a')
	}, hex.EncodeToString(h[:16]))
	if id != arquivo.IDChrome {
		t.Fatalf("o ID derivado da chave é %s, o arquivo diz %s", id, arquivo.IDChrome)
	}
	if !slices.Equal(extensoesChromeDev, []string{id}) {
		t.Fatalf("o build dev aceita %v, a chave dá %s", extensoesChromeDev, id)
	}
	if !(Chamador{Navegador: Chromium, Extensao: id}).Permitido() || !slices.Contains(ExtensoesChrome(), id) {
		t.Fatal("o build dev não aceita a própria extensão de desenvolvimento")
	}
}
