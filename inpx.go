// Package inpx provides a set of methods to read .inpx files (book library index).
package inpx

import (
	"archive/zip"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// Known fields for inp files.
const (
	FieldAuthor = iota
	FieldGenre
	FieldTitle
	FieldSeries
	FieldSeriesNum
	FieldFileName
	FieldFileSize
	FieldLibId
	FieldDeleted
	FieldExt
	FieldDate
	FieldLang
	FieldLibRate
	FieldKeywords
)

// DefaultStructure is an inp file field order used by default.
var DefaultStructure = []int{
	FieldAuthor, FieldGenre, FieldTitle, FieldSeries, FieldSeriesNum,
	FieldFileName, FieldFileSize, FieldLibId, FieldDeleted, FieldExt, FieldDate, FieldLang,
	FieldLibRate, FieldKeywords,
}

func splitBy(s string, r rune) (out []string) {
	arr := []rune(strings.TrimRight(s, string(r)))
	last := 0
	for i := 0; i < len(arr); i++ {
		if arr[i] == r {
			if i+1 < len(arr) && arr[i+1] != ' ' {
				out = append(out, string(arr[last:i]))
			}
			last = i + 1
		}
	}
	if last < len(arr) {
		out = append(out, string(arr[last:]))
	}
	return
}

func splitName(s string) []string {
	return splitBy(s, ',')
}

func fieldsToBook(fields [][]byte, structure []int) (Book, error) {
	if len(fields) < len(structure) {
		return Book{}, fmt.Errorf("wrong fields count: %d", len(fields))
	}
	var errg error
	toStr := func() string {
		cur := fields[0]
		if len(cur) > 0 && cur[len(cur)-1] == ':' {
			cur = cur[:len(cur)-1]
		}
		s := strings.TrimSpace(string(cur))
		fields = fields[1:]
		return s
	}
	toInt := func() int {
		s := toStr()
		if s == "" {
			return 0
		}
		v, err := strconv.Atoi(s)
		if err != nil && errg == nil {
			errg = err
		}
		return v
	}
	toDate := func() time.Time {
		s := toStr()
		if s == "" {
			return time.Time{}
		}
		v, err := time.Parse("2006-01-02", s)
		if err != nil && errg == nil {
			errg = err
		}
		return v
	}
	fieldMap := make(map[int]interface{})
	for _, f := range structure {
		var v interface{}
		switch f {
		case FieldAuthor:
			var authors []Author
			for _, name := range strings.Split(toStr(), ":") {
				a := Author{Name: splitName(name)}
				for i := range a.Name {
					a.Name[i] = strings.TrimSpace(a.Name[i])
				}
				authors = append(authors, a)
			}
			v = authors
		case FieldGenre:
			v = strings.Split(toStr(), ":")
		case FieldDeleted:
			v = toInt() != 0
		case FieldDate:
			v = toDate()
		case FieldSeriesNum, FieldFileSize, FieldLibId:
			v = toInt()
		case FieldKeywords:
			v = strings.Split(toStr(), ",")
		default:
			v = toStr()
		}
		fieldMap[f] = v
	}
	setField := func(f int, dest interface{}) {
		if v := fieldMap[f]; v != nil {
			reflect.ValueOf(dest).Elem().Set(reflect.ValueOf(v))
		}
	}
	var record Book
	setField(FieldAuthor, &record.Authors)
	setField(FieldGenre, &record.Genres)
	setField(FieldTitle, &record.Title)
	setField(FieldSeries, &record.Series)
	setField(FieldSeriesNum, &record.SeriesNum)
	setField(FieldFileName, &record.File.Name)
	setField(FieldFileSize, &record.File.Size)
	setField(FieldExt, &record.File.Ext)
	setField(FieldLibId, &record.LibId)
	setField(FieldDeleted, &record.Deleted)
	setField(FieldDate, &record.Date)
	setField(FieldLang, &record.Lang)
	return record, errg
}

// OpenWithStructure reads whole library index from an inpx file
// using a provided field structure for individual inp files.
func OpenWithStructure(path string, structure []int) (*Index, error) {
	zf, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zf.Close()

	dir := filepath.Dir(path)
	index := &Index{
		Archives: make(map[string][]Book),
	}
	total := 0
	for _, f := range zf.File {
		switch f.Name {
		case "version.info":
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("error while reading version info: %v", err)
			}
			_, err = fmt.Fscan(rc, &index.Version)
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("error while reading version info: %v", err)
			}
		case "collection.info":
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("error while reading collection info: %v", err)
			}
			br := bufio.NewReader(rc)
			index.Name, err = br.ReadString('\n')
			index.Name = strings.Trim(index.Name, "\n\t \ufeff")
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("error while reading collection info: %v", err)
			}
		default:
			if !strings.HasSuffix(f.Name, ".inp") {
				log.Println("unknown file:", f.Name)
				continue
			}
			pack := strings.TrimSuffix(f.Name, ".inp")
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("error while reading inp: %v", err)
			}
			br := bufio.NewReader(rc)
			var recs []Book
			for {
				line, err := br.ReadBytes('\n')
				if err == io.EOF {
					break
				}
				if err != nil {
					rc.Close()
					return nil, fmt.Errorf("error while reading inp: %v", err)
				}
				if len(line) > 0 {
					line = line[:len(line)-1]
				}
				rec, err := fieldsToBook(bytes.Split(line, []byte{0x04}), structure)
				if err != nil {
					log.Println("err:", err)
				} else {
					rec.File.Dir = dir
					rec.File.Archive = pack
					recs = append(recs, rec)
				}
			}
			rc.Close()
			{
				nrec := make([]Book, len(recs))
				copy(nrec, recs)
				recs = nrec
			}
			index.Archives[pack] = recs
			total += len(recs)
		}
	}
	return index, nil
}

// Open reads whole library index from an inpx file.
func Open(path string) (*Index, error) {
	return OpenWithStructure(path, DefaultStructure)
}

// Index describes an inpx file information.
type Index struct {
	Name     string
	Version  int
	Archives map[string][]Book
}

type multiReadCloser struct {
	io.Reader
	closers []io.Closer
}

func (mc multiReadCloser) Close() error {
	for _, c := range mc.closers {
		c.Close()
	}
	return nil
}

// Author describes an author.
type Author struct {
	Name []string
}

// File describes a book file in archive.
type File struct {
	Name    string
	Ext     string
	Dir     string
	Archive string
	Size    int
}

// Open opens a book file from archive.
func (fr File) Open() (io.ReadCloser, error) {
	zfile, err := zip.OpenReader(filepath.Join(fr.Dir, fr.Archive+".zip"))
	if err != nil {
		return nil, err
	}
	for _, f := range zfile.File {
		if f.Name == fr.Name+"."+fr.Ext {
			file, err := f.Open()
			if err != nil {
				zfile.Close()
				return nil, err
			}
			return multiReadCloser{
				Reader:  file,
				closers: []io.Closer{file, zfile},
			}, nil
		}
	}
	zfile.Close()
	return nil, os.ErrNotExist
}

// Book describes a book in archive.
type Book struct {
	Authors   []Author
	Genres    []string
	Title     string
	Series    string
	SeriesNum int
	File      File
	LibId     int
	Deleted   bool
	Date      time.Time
	Lang      string
	//Keywords  []string
}
