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

package tools

import (
	"strings"
	"testing"
)

// TestValidateAppendInput_RejectsContentOverMaxContentSize verifies that
// validateAppendInput rejects content larger than maxContentSize.
func TestValidateAppendInput_RejectsContentOverMaxContentSize(t *testing.T) {
	oversized := strings.Repeat("x", maxContentSize+1)
	if _, err := validateAppendInput("notes", oversized); err == nil {
		t.Fatal("expected an error for content exceeding maxContentSize")
	}
}

// TestValidateAppendInput_AllowsContentAtMaxContentSize verifies that
// validateAppendInput accepts content of exactly maxContentSize bytes.
func TestValidateAppendInput_AllowsContentAtMaxContentSize(t *testing.T) {
	atLimit := strings.Repeat("x", maxContentSize)
	if _, err := validateAppendInput("notes", atLimit); err != nil {
		t.Fatalf("expected content exactly at maxContentSize to be accepted, got: %v", err)
	}
}

// TestValidateWriteInput_RejectsContentOverMaxContentSize verifies that
// validateWriteInput rejects content larger than maxContentSize.
func TestValidateWriteInput_RejectsContentOverMaxContentSize(t *testing.T) {
	oversized := strings.Repeat("x", maxContentSize+1)
	if _, err := validateWriteInput("notes", oversized); err == nil {
		t.Fatal("expected an error for content exceeding maxContentSize")
	}
}

// TestValidateWriteInput_AllowsContentAtMaxContentSize verifies that
// validateWriteInput accepts content of exactly maxContentSize bytes.
func TestValidateWriteInput_AllowsContentAtMaxContentSize(t *testing.T) {
	atLimit := strings.Repeat("x", maxContentSize)
	if _, err := validateWriteInput("notes", atLimit); err != nil {
		t.Fatalf("expected content exactly at maxContentSize to be accepted, got: %v", err)
	}
}
