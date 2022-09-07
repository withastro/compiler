package astro

import (
	"encoding/base32"
	"strings"

	"github.com/withastro/compiler/internal/xxhash"
)

// This is used in `Transform` to ensure a stable hash when updating styles
func HashFromDoc(doc *Node, moduleId string) string {
	var b strings.Builder
	PrintToSource(&b, doc)
	source := strings.TrimSpace(b.String())
	return HashFromSourceAndModuleId(source, moduleId)
}

func HashFromSourceAndModuleId(source, moduleId string) string {
	h := xxhash.New()
	//nolint
	h.Write([]byte(moduleId + source))
	hashBytes := h.Sum(nil)
	return base32.StdEncoding.EncodeToString(hashBytes)[:8]
}

func HashFromSource(source string) string {
	return HashFromSourceAndModuleId(source, "")
}
