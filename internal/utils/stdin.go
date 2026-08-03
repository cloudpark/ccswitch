package utils

import (
	"bufio"
	"os"
	"strings"
)

// stdinReader is a single shared reader over os.Stdin. Any prompt that reads
// a line of user input during a command's execution (e.g. the session
// description prompt, the post-create trust prompt) must go through
// ReadStdinLine rather than creating its own bufio.Scanner/Reader - a second
// independent reader over os.Stdin can silently lose input already buffered
// ahead by the first one.
var stdinReader = bufio.NewReader(os.Stdin)

// ReadStdinLine reads a single line from stdin, trimmed of surrounding
// whitespace. ok is false if no line could be read (e.g. stdin is closed or
// at EOF with nothing left to read).
func ReadStdinLine() (line string, ok bool) {
	text, err := stdinReader.ReadString('\n')
	if err != nil && text == "" {
		return "", false
	}
	return strings.TrimSpace(text), true
}
