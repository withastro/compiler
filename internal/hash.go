package astro

import (
	"encoding/base32"
	"strings"

	"github.com/withastro/compiler/internal/xxhash"
)

func HashString(str string) string {
	h := xxhash.New()
	//nolint
	h.Write([]byte(str))
	hashBytes := h.Sum(nil)
	return strings.ToLower(base32.StdEncoding.EncodeToString(hashBytes)[:8])
}
