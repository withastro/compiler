package test_utils

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/google/go-cmp/cmp"
	"github.com/lithammer/dedent"
)

func RemoveNewlines(input string) string {
	return strings.ReplaceAll(input, "\n", "")
}

func Dedent(input string) string {
	return dedent.Dedent( // removes any leading whitespace
		strings.ReplaceAll( // compress linebreaks to 1 or 2 lines max
			strings.TrimLeft(
				strings.TrimRight(input, " \n\r"), // remove any trailing whitespace
				" \t\r\n"),                        // remove leading whitespace
			"\n\n\n", "\n\n"),
	)
}

func ANSIDiff(x, y interface{}, opts ...cmp.Option) string {
	escapeCode := func(code int) string {
		return fmt.Sprintf("\x1b[%dm", code)
	}
	diff := cmp.Diff(x, y, opts...)
	if diff == "" {
		return ""
	}
	ss := strings.Split(diff, "\n")
	for i, s := range ss {
		switch {
		case strings.HasPrefix(s, "-"):
			ss[i] = escapeCode(31) + s + escapeCode(0)
		case strings.HasPrefix(s, "+"):
			ss[i] = escapeCode(32) + s + escapeCode(0)
		}
	}
	return strings.Join(ss, "\n")
}

// Removes unsupported characters from the test case name, because it will be used as name for the snapshot
func RedactTestName(testCaseName string) string {
	snapshotName := strings.ReplaceAll(testCaseName, "#", "_")
	snapshotName = strings.ReplaceAll(snapshotName, "<", "_")
	snapshotName = strings.ReplaceAll(snapshotName, ">", "_")
	snapshotName = strings.ReplaceAll(snapshotName, ")", "_")
	snapshotName = strings.ReplaceAll(snapshotName, "(", "_")
	snapshotName = strings.ReplaceAll(snapshotName, ":", "_")
	snapshotName = strings.ReplaceAll(snapshotName, " ", "_")
	snapshotName = strings.ReplaceAll(snapshotName, "#", "_")
	snapshotName = strings.ReplaceAll(snapshotName, "'", "_")
	snapshotName = strings.ReplaceAll(snapshotName, "\"", "_")
	snapshotName = strings.ReplaceAll(snapshotName, "@", "_")
	snapshotName = strings.ReplaceAll(snapshotName, "`", "_")
	snapshotName = strings.ReplaceAll(snapshotName, "+", "_")
	return snapshotName
}

type OutputKind int

const (
	JsOutput = iota
	JsonOutput
	CssOutput
	HtmlOutput
	JsxOutput
)

var outputKind = map[OutputKind]string{
	JsOutput:   "js",
	JsonOutput: "json",
	CssOutput:  "css",
	HtmlOutput: "html",
	JsxOutput:  "jsx",
}

type SnapshotOptions struct {
	Testing      *testing.T
	TestCaseName string
	Input        string
	Output       string
	Kind         OutputKind
	FolderName   string
}

// It creates a snapshot for the given test case, the snapshot will include the input and the output of the test case
func MakeSnapshot(options *SnapshotOptions) {
	t := options.Testing
	testCaseName := options.TestCaseName
	input := options.Input
	output := options.Output
	kind := options.Kind

	folderName := "__snapshots__"
	if options.FolderName != "" {
		folderName = options.FolderName
	}
	snapshotName := RedactTestName(testCaseName)

	s := snaps.WithConfig(
		snaps.Filename(snapshotName),
		snaps.Dir(folderName),
	)

	snapshot := "## Input\n\n```\n"
	snapshot += Dedent(input)
	snapshot += "\n```\n\n## Output\n\n"
	snapshot += "```" + outputKind[kind] + "\n"
	snapshot += Dedent(output)
	snapshot += "\n```"

	s.MatchSnapshot(t, snapshot)

}
