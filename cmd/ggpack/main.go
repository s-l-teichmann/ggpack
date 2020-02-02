package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/s-l-teichmann/ggpack"
)

var listFiles = true

func process(fname string) error {
	file, err := os.Open(fname)
	if err != nil {
		return err
	}
	defer file.Close()

	reader := ggpack.Reader{Reader: file}
	if err := reader.ReadPack(); err != nil {
		return err
	}

	if listFiles {
		files := reader.Entries().Find("files")
		if files == nil || files.Type() != ggpack.ArrayType {
			return errors.New("no files found")
		}
		stdout := bufio.NewWriter(os.Stdout)
		fs := files.Array()
		for _, f := range fs {
			if f.Type() != ggpack.HashType {
				continue
			}
			name := f.Find("filename")
			size := f.Find("size")
			if name != nil && size != nil &&
				name.Type() == ggpack.StringType &&
				size.Type() == ggpack.IntegerType {
				fmt.Fprintf(stdout, "%s\t%d\n", name.String(), size.Integer())
			}
		}
		if err := stdout.Flush(); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	flag.BoolVar(&listFiles, "list", true, "list all files")
	flag.Parse()

	for _, arg := range flag.Args() {
		if err := process(arg); err != nil {
			log.Fatalf("error processing %s: %v\n", arg, err)
		}
	}
}
