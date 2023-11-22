package helpers

import (
	"errors"
	"strings"
)

// Remove comment blocks from string (e.g. "/* a comment */aProp" => "aProp")
func removeBlockComments(input string) (string, error) {
	var (
		sb        = strings.Builder{}
		inComment = false
	)
	for cur := 0; cur < len(input); cur++ {
		peekIs := func(assert byte) bool { return cur+1 < len(input) && input[cur+1] == assert }
		if input[cur] == '/' && !inComment && peekIs('*') {
			inComment = true
			cur++
		} else if input[cur] == '*' && inComment && peekIs('/') {
			inComment = false
			cur++
		} else if !inComment {
			sb.WriteByte(input[cur])
		}
	}

	if inComment {
		return "", errors.New("unterminated comment")
	}

	return strings.TrimSpace(sb.String()), nil

	// ##########################################################

	// var cleanedInput bytes.Buffer
	// inComment := false

	// // Remove multiline comments
	// multilineCommentRegex := regexp.MustCompile(`/\*.*?\*/`)
	// input = multilineCommentRegex.ReplaceAllStringFunc(input, func(match string) string {
	// 	inComment = !inComment
	// 	return ""
	// })

	// if inComment {
	// 	return "", errors.New("unterminated comment")
	// }

	// // Remove inline comments
	// inlineCommentRegex := regexp.MustCompile(`//.*?(?:\n|$)`)
	// input = inlineCommentRegex.ReplaceAllString(input, "")

	// // Append the cleaned JSX to the buffer
	// cleanedInput.WriteString(input)

	// return strings.TrimSpace(input), nil
}

func removeInlineComments(input string) (string, error) {
	var (
		sb        = strings.Builder{}
		inComment = false
	)
	for cur := 0; cur < len(input); cur++ {
		peekIs := func(assert byte) bool { return cur+1 < len(input) && input[cur+1] == assert }
		if input[cur] == '/' && !inComment && peekIs('/') {
			inComment = true
			cur++
		} else if input[cur] == '\n' && inComment {
			inComment = false
		} else if !inComment {
			sb.WriteByte(input[cur])
		}
	}

	if inComment {
		return "", errors.New("unterminated comment")
	}
	return strings.TrimSpace(input), nil

	// return removeBlockComments(input)
}

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
