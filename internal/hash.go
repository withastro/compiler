package astro

import (
	"encoding/base32"
	"strings"

	"github.com/withastro/compiler/internal/xxhash"
)

// This is used in `Transform` to ensure a stable hash when updating styles
func HashFromDoc(doc *Node) string {
	var b strings.Builder
	PrintToSource(&b, doc)
	source := strings.TrimSpace(b.String())
	return HashFromSource(source)
}

func HashFromSource(source string) string {
	h := xxhash.New()
	//nolint
	h.Write([]byte(source))
	hashBytes := h.Sum(nil)
	return base32.StdEncoding.EncodeToString(hashBytes)[:8]
}
