package helpers

import (
	"errors"
	"strings"
)

// RemoveComments removes both block and inline comments from a string
func RemoveComments(input string) (string, error) {
	var (
		sb        = strings.Builder{}
		inComment = false
	)
	for cur := 0; cur < len(input); cur++ {
		peekIs := func(assert byte) bool { return cur+1 < len(input) && input[cur+1] == assert }

		if input[cur] == '/' && !inComment {
			if peekIs('*') {
				inComment = true
				cur++
			} else if peekIs('/') {
				// Skip until the end of line for inline comments
				for cur < len(input) && input[cur] != '\n' {
					cur++
				}
				continue
			}
		} else if input[cur] == '*' && inComment && peekIs('/') {
			inComment = false
			cur++
			continue
		}

		if !inComment {
			sb.WriteByte(input[cur])
		}
	}

	if inComment {
		return "", errors.New("unterminated comment")
	}

	return strings.TrimSpace(sb.String()), nil
}
