package astro

import (
	"encoding/base32"

	"github.com/withastro/compiler/internal/xxhash"
)

func HashFromSource(source string) string {
	h := xxhash.New()
	//nolint
	h.Write([]byte(source))
	hashBytes := h.Sum(nil)
	return base32.StdEncoding.EncodeToString(hashBytes)[:8]
}
