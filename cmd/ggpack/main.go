package main

import (
	"flag"
	"log"
	"os"

	"github.com/s-l-teichmann/ggpack"
)

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
	return nil
}

func main() {
	flag.Parse()

	for _, arg := range flag.Args() {
		if err := process(arg); err != nil {
			log.Fatalf("error processing %s: %v\n", arg, err)
		}
	}
}
