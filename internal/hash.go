package astro

import (
	"encoding/base32"
	"strings"

	"github.com/withastro/compiler/internal/xxhash"
)

// This is used in `Transform` to ensure a stable hash when updating styles
func HashFromDoc(doc *Node, filename string) string {
	var b strings.Builder
	PrintToSource(&b, doc)
	source := strings.TrimSpace(b.String())
	return HashFromSource(source, filename)
}

func HashFromSource(source string, filename string) string {
	h := xxhash.New()
	//nolint
	h.Write([]byte(filename + source))
	hashBytes := h.Sum(nil)
	return base32.StdEncoding.EncodeToString(hashBytes)[:8]
}
