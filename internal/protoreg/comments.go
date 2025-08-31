package protoreg

import (
	"strings"

	"github.com/jhump/protoreflect/v2/protobuilder"
)

func comment(desc string) protobuilder.Comments {
	if desc == "" {
		return protobuilder.Comments{}
	}
	// Prefix each line with a space and ensure trailing newline.
	lines := strings.Split(desc, "\n")
	for i, line := range lines {
		lines[i] = " " + line
	}
	return protobuilder.Comments{LeadingComment: strings.Join(lines, "\n") + "\n"}
}
