package astro

import (
	"encoding/base32"

	"github.com/withastro/compiler/internal/xxhash"
)

func HashString(str string) string {
	h := xxhash.New()
	//nolint
	h.Write([]byte(str))
	hashBytes := h.Sum(nil)
	return base32.StdEncoding.EncodeToString(hashBytes)[:8]
}
