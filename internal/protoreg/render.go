package protoreg

import (
	"os"
	"path"

	"github.com/jhump/protoreflect/v2/protoprint"
)

// Render generates proto definitions based on the provided registry and outputs them to the specified directory.
func Render(r *Registry, outDir string) error {
	pp := protoprint.Printer{}

	for _, fd := range r.GetAllServiceFiles() {
		fp := path.Join(outDir, fd.Path())
		if err := os.MkdirAll(path.Dir(fp), 0755); err != nil {
			return err
		}
		openedFile, err := os.OpenFile(fp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		defer openedFile.Close()

		if err = pp.PrintProtoFile(fd, openedFile); err != nil {
			return err
		}
	}
	return nil
}
