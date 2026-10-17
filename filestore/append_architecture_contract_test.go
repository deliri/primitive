package filestore

import (
	"go/parser"
	"go/token"
	"testing"
)

func TestFilestoreArchitectureRecognizesGoBackedAppendCustody(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		want         int
	}{
		{name: "private native file and Go buffer delegate effects", source: `package filestore
import (native "os"; window "bufio")
type owner struct{self *owner; file *native.File; writer *window.Writer}
func(d *owner) Write(p []byte)(int,error){return d.writer.Write(p)}
func(d *owner) Sync()error{return d.file.Sync()}
func(d *owner) Close()error{return d.settle()}
func(d *owner) settle()error{return d.file.Close()}`},
		{name: "lookalike without Go custody remains refused", source: `package filestore
type owner struct{}
func(d *owner) Write(p []byte)(int,error){return len(p),nil}
func(d *owner) Sync()error{return nil}
func(d *owner) Close()error{return nil}`, want: 3},
		{name: "owning native field cannot bless fabricated writes", source: `package filestore
import "os"
type owner struct{file *os.File}
func(d *owner) Write(p []byte)(int,error){return len(p),nil}`, want: 1},
		{name: "foreign File spelling confers no native custody", source: `package filestore
import os "example.com/foreign"
type owner struct{file *os.File}
func(d *owner) Sync()error{return d.file.Sync()}`, want: 1},
		{name: "borrowed native argument is not owned custody", source: `package filestore
import "os"
type owner struct{}
func(d *owner) Sync(file *os.File)error{return file.Sync()}`, want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", tc.source, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			got, err := scanProductionArchitecture([]productionFile{{file: file, name: "fixture.go"}})
			if err != nil || len(got.violations) != tc.want {
				t.Fatalf("violations=%v,%v,want %d", got.violations, err, tc.want)
			}
		})
	}
}
