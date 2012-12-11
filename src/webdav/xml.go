package webdav

import (
	"encoding/xml"
	"fmt"
	"io"
)

type XmlError string

func parsexml(r io.Reader) *xmlparser {
	x := &xmlparser{p: xml.NewDecoder(r)}
	x.next()
	return x
}

type xmlparser struct {
	p   *xml.Decoder
	cur xml.Token
}

// next moves to the next token,
// skipping anything that is not an element
// in the DAV: namespace
func (x *xmlparser) next() xml.Token {
	var err error
	for {
		x.cur, err = x.p.Token()
		if err == io.EOF {
			return x.cur
		} else if err != nil {
			panic(XmlError("error fetching token"))
		}
		switch tok := x.cur.(type) {
		case xml.StartElement:
			if tok.Name.Space != "DAV:" {
				err = x.p.Skip()
				if err != nil && err != io.EOF {
					panic(XmlError("error skipping other namespace"))
				}
			} else {
				return x.cur
			}
		case xml.EndElement:
			return x.cur
		}
	}
	panic("unreachable")
}

func (x *xmlparser) start(name string) bool {
	el, ok := x.cur.(xml.StartElement)
	if !ok {
		panic(XmlError("can't cast cur"))
	}

	if el.Name.Local != name {
		return false
	}
	x.next()
	return true
}

func (x *xmlparser) mustStart(name string) {
	if !x.start(name) {
		el, _ := x.cur.(xml.StartElement)
		panic(XmlError(fmt.Sprint("expected ", name, " but got ", el.Name.Local)))
	}
}

func (x *xmlparser) end(name string) bool {
	if _, ok := x.cur.(xml.EndElement); !ok {
		// todo: validate
		panic(XmlError("can't cast cur"))
	}
	x.next()
	return true
}

func (x *xmlparser) mustEnd(name string) {
	if !x.end(name) {
		el, _ := x.cur.(xml.StartElement)
		panic(fmt.Sprint("expected ", name, " but got ", el.Name.Local))
	}
}
