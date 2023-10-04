package printer

import (
	"strings"
	"testing"

	"github.com/withastro/compiler/internal/test_utils"
)

type paramsTestcase struct {
	name string
	want string
}

func TestUtilParamsType(t *testing.T) {
	tests := []paramsTestcase{
		{
			name: "/src/pages/index.astro",
			want: `Record<string, string | number>`,
		},
		{
			name: "/src/pages/blog/[slug].astro",
			want: `Record<"slug", string | number>`,
		},
		{
			name: "/src/pages/[lang]/blog/[slug].astro",
			want: `Record<"lang" | "slug", string | number>`,
		},
		{
			name: "/src/pages/[...fallback].astro",
			want: `Record<"fallback", string | number>`,
		},
		{
			name: "/src/pages/[year]-[month]-[day]/[post].astro",
			want: `Record<"year" | "month" | "day" | "post", string | number>`,
		},
		{
			name: "/src/pages/post-[id]/[post].astro",
			want: `Record<"id" | "post", string | number>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getParamsTypeFromFilename(tt.name)
			// compare to expected string, show diff if mismatch
			if diff := test_utils.ANSIDiff(strings.TrimSpace(tt.want), strings.TrimSpace(string(result))); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
