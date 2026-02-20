package iif

import (
	"encoding/csv"
	"errors"
	"io"
	"strings"
)

var (
	ErrInvalidHeaderLine     = errors.New("iif: invalid header line")
	ErrMismatchedColumns     = errors.New("iif: mismatched number of columns")
	ErrMismatchedRecords     = errors.New("iif: row does not match expected header")
	ErrUnknownRecordType     = errors.New("iif: unknown record type")
	ErrUnexpectedSectionType = errors.New("iif: unexpected record type for current section")
)

type RecordType string

type Header struct {
	Type   RecordType
	Fields []string
}

type Record struct {
	Type   RecordType
	Fields map[string]string
}

type Records []Record

type Block struct {
	Header    []Header
	Records   Records
	curHeader int
}

type File struct {
	Blocks []*Block
}

type Decoder struct {
	r *csv.Reader
}

func NewDecoder(r io.Reader) *Decoder {
	reader := csv.NewReader(r)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = false
	reader.FieldsPerRecord = -1
	return &Decoder{r: reader}
}

func (f *File) NewBlock() *Block {
	f.Blocks = append(f.Blocks, &Block{})

	return f.Blocks[len(f.Blocks)-1]
}

func (b *Block) AddHeader(h Header) {
	b.Header = append(b.Header, h)
}

func (b *Block) AddRecord(t RecordType, values []string) error {
	header := b.Header[b.curHeader]
	for t != header.Type {
		if b.curHeader += 1; b.curHeader >= len(b.Header) {
			return ErrMismatchedRecords
		}
		header = b.Header[b.curHeader]
	}

	r := Record{
		Type:   t,
		Fields: make(map[string]string, len(values)),
	}
	for i, h := range header.Fields {
		r.Fields[h] = values[i]
	}

	b.Records = append(b.Records, r)
	return nil
}

func trimLine(records []string) []string {
	for i, r := range records {
		if r == "" {
			return records[:i]
		}
	}
	return records
}

func (d *Decoder) Decode() (*File, error) {
	f := File{}
	var b *Block

	parsingHeaders := false
	for {
		record, err := d.r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		key := record[0]
		if strings.HasPrefix(key, "!") {
			if !parsingHeaders {
				b = f.NewBlock()
			}

			parsingHeaders = true
			b.AddHeader(Header{
				Type:   RecordType(key[1:]),
				Fields: trimLine(record[1:]),
			})
		} else {
			parsingHeaders = false
			err := b.AddRecord(RecordType(key), record[1:])
			if err != nil {
				return nil, err
			}
		}
	}

	return &f, nil
}

type Encoder struct {
	w io.Writer
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

func (e *Encoder) Encode(f *File) error {
	return nil
}
