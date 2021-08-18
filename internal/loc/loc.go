package loc

type Loc struct {
	// This is the 0-based index of this location from the start of the file, in bytes
	Start int
}

type Range struct {
	Loc Loc
	Len int
}

func (r Range) End() int {
	return r.Loc.Start + r.Len
}

// span is a range of bytes in a Tokenizer's buffer. The start is inclusive,
// the end is exclusive.
type Span struct {
	Start, End int
}
