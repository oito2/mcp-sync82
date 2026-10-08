// Copyright (C) 2026  OITO2
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Package prompt asks the yes/no confirmation questions of the CLI
// commands. A question gives up as soon as its context ends — a first
// Ctrl-C — instead of waiting for an answer that will never come.
package prompt

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrNoAnswer is returned by AskYes when its input ends before any answer
// is read, as with a closed stdin or one redirected from /dev/null.
var ErrNoAnswer = errors.New("no answer to the confirmation prompt: stdin is closed")

// AskYes prints question to out and reports whether the next line read
// from in is "y" or "yes", ignoring case and surrounding spaces. It
// returns ErrNoAnswer when in ends before a non-empty answer, ctx's error
// when ctx ends first (the read itself keeps waiting in the background,
// which is harmless as the process is about to exit), or the read error.
func AskYes(ctx context.Context, in *bufio.Reader, out io.Writer, question string) (bool, error) {
	fmt.Fprint(out, question)
	type result struct {
		line string
		err  error
	}
	read := make(chan result, 1)
	go func() {
		line, err := in.ReadString('\n')
		read <- result{line, err}
	}()
	var r result
	select {
	case r = <-read:
	case <-ctx.Done():
		fmt.Fprintln(out)
		return false, ctx.Err()
	}
	answer := strings.ToLower(strings.TrimSpace(r.line))
	if r.err != nil && answer == "" {
		fmt.Fprintln(out)
		if errors.Is(r.err, io.EOF) {
			return false, ErrNoAnswer
		}
		return false, r.err
	}
	return answer == "y" || answer == "yes", nil
}
