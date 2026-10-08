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

package prompt

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// TestAskYes checks the answers counted as yes, the error of an input that
// ends without an answer, and that the question is printed.
func TestAskYes(t *testing.T) {
	for input, want := range map[string]bool{"y\n": true, " YES \n": true, "n\n": false, "\n": false, "yes": true} {
		var out strings.Builder
		got, err := AskYes(context.Background(), bufio.NewReader(strings.NewReader(input)), &out, "Go? ")
		if err != nil || got != want {
			t.Errorf("AskYes(%q) = %v, %v; want %v", input, got, err, want)
		}
		if !strings.HasPrefix(out.String(), "Go? ") {
			t.Errorf("printed %q, want the question", out.String())
		}
	}
	if _, err := AskYes(context.Background(), bufio.NewReader(strings.NewReader("")), io.Discard, "Go? "); !errors.Is(err, ErrNoAnswer) {
		t.Errorf("closed input: err = %v, want ErrNoAnswer", err)
	}
}

// TestAskYes_StopsWhenTheContextEnds checks that a question waiting on an
// input that never answers returns the context's error as soon as the
// context is cancelled, as a Ctrl-C does.
func TestAskYes_StopsWhenTheContextEnds(t *testing.T) {
	r, w := io.Pipe()
	defer w.Close()
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	_, err := AskYes(ctx, bufio.NewReader(r), io.Discard, "Go? ")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("AskYes returned %s after the cancellation", elapsed)
	}
}
