package ir

import (
	"fmt"

	language "github.com/hanpama/protograph/internal/language"
)

type Violation struct {
	Message string `json:"message"`
	File    string `json:"file,omitempty"`
	Line    int    `json:"positionStart,omitempty"`
	Column  int    `json:"positionEnd,omitempty"`
}

type ValidationError []*Violation

func (e ValidationError) Error() string {
	msg := "violations found:\n"
	for _, v := range e {
		line := "- " + v.Message
		if v.File != "" {
			line += fmt.Sprintf(" %s:%d:%d", v.File, v.Line, v.Column)
		}
		msg += line + "\n"
	}
	return msg
}

// Core primitive used by all template helpers.
func violationWithPosition(message string, pos *language.Position) *Violation {
	return &Violation{
		Message: message,
		File:    pos.Src.Name,
		Line:    pos.Line,
		Column:  pos.Column,
	}
}
