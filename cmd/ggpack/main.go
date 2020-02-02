package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/s-l-teichmann/ggpack"
)

var (
	extractFiles = ""
	dir          = "."
)

func handleFiles(
	reader *ggpack.Reader,
	fn func(name string, ofs, size int64) error,
) error {

	files := reader.Entries().Find("files")
	if files == nil || files.Type() != ggpack.ArrayType {
		return errors.New("no files found")
	}

	fs := files.Array()
	for _, f := range fs {
		if f.Type() != ggpack.HashType {
			continue
		}
		name := f.Find("filename")
		ofs := f.Find("offset")
		size := f.Find("size")
		if name != nil && ofs != nil && size != nil &&
			name.Type() == ggpack.StringType &&
			ofs.Type() == ggpack.IntegerType &&
			size.Type() == ggpack.IntegerType {
			if err := fn(name.String(), ofs.Integer(), size.Integer()); err != nil {
				return err
			}
		}
	}

	return nil
}

func loadIndex(fname string) (*ggpack.Reader, error) {

	file, err := os.Open(fname)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := ggpack.Reader{Reader: file}
	if err := reader.ReadPack(); err != nil {
		return nil, err
	}
	return &reader, nil
}

func process(fname string) error {

	index, err := loadIndex(fname)
	if err != nil {
		return err
	}

	if extractFiles == "" {
		stdout := bufio.NewWriter(os.Stdout)
		if err := handleFiles(index, func(name string, _, size int64) error {
			_, err := fmt.Fprintf(stdout, "%s\t%d\n", name, size)
			return err
		}); err != nil {
			return err
		}
		return stdout.Flush()
	}

	re, err := regexp.Compile(extractFiles)
	if err != nil {
		return err
	}

	return func() error {
		file, err := os.Open(fname)
		if err != nil {
			return err
		}
		defer file.Close()
		var buf []byte
		return handleFiles(index, func(name string, ofs, size int64) error {
			if !re.MatchString(name) {
				return nil
			}
			if _, err := file.Seek(ofs, io.SeekStart); err != nil {
				return err
			}
			if int64(cap(buf)) >= size {
				buf = buf[:size]
			} else {
				buf = make([]byte, size)
			}
			_, err := io.ReadFull(file, buf)
			if err != nil {
				return err
			}
			index.DecodeXOR(buf)
			if strings.HasSuffix(strings.ToLower(name), ".bnut") {
				ggpack.DecodeBnut(buf)
				// remove trailing zeros.
				buf = trimZeros(buf)
			}
			fname := filepath.Join(dir, name)
			return ioutil.WriteFile(fname, buf, 0666)
		})
	}()
}

func trimZeros(buf []byte) []byte {
	for len(buf) > 0 && buf[len(buf)-1] == 0 {
		buf = buf[:len(buf)-1]
	}
	return buf
}

func main() {
	flag.StringVar(&dir, "dir", ".", "directory to extract files to")
	flag.StringVar(&extractFiles, "extract", "", "pattern of files to files")
	flag.Parse()

	for _, arg := range flag.Args() {
		if err := process(arg); err != nil {
			log.Fatalf("error processing %s: %v\n", arg, err)
		}
	}
}
