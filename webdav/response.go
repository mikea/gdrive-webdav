package webdav

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"net/http"
	"text/template"
	"time"
)

type xmlSerializable interface {
	XML(b *bytes.Buffer)
}

// 14.7 href XML Element
type href string

func (h *href) XML(b *bytes.Buffer) {
	b.WriteString("<href>" + template.HTMLEscapeString(string(*h)) + "</href>")
}

// 14.16 multistatus XML Element
type multistatus []*response

func (m multistatus) XML(b *bytes.Buffer) {
	b.WriteString("<multistatus xmlns='DAV:'>")
	for _, r := range m {
		r.XML(b)
	}
	b.WriteString("</multistatus>")
}

// 14.24 response XML Element
type response struct {
	href href
	body xmlSerializable
}

func (r *response) XML(b *bytes.Buffer) {
	b.WriteString("<response>")
	r.href.XML(b)
	r.body.XML(b)
	b.WriteString("</response>")
}

// part of 14.24 response XML element

type hrefsstatus struct {
	hrefs  []*href
	status status
}

func (hs *hrefsstatus) XML(b *bytes.Buffer) {
	for _, h := range hs.hrefs {
		h.XML(b)
	}
	hs.status.XML(b)
}

// part of 14.24 response element

type propstats []propstat

func (p propstats) XML(b *bytes.Buffer) {
	b.WriteString("<propstat>")
	for _, prop := range p {
		prop.XML(b)
	}
	b.WriteString("</propstat>")
}

// 14.22 propstat XML Element
type propstat struct {
	props  []xmlSerializable
	status status
}

func (p *propstat) XML(b *bytes.Buffer) {
	b.WriteString("<prop>")
	for _, prop := range p.props {
		prop.XML(b)
	}
	b.WriteString("</prop>")
	p.status.XML(b)
}

// 14.28 status XML element
type status int

func (s status) XML(b *bytes.Buffer) {
	b.WriteString(fmt.Sprintf("<status>HTTP/1.1 %d %s</status>", s, template.HTMLEscapeString(http.StatusText(int(s)))))
}

func (c CreationDatePropertyValue) XML(b *bytes.Buffer) {
	b.WriteString("<creationdate>")
	b.WriteString(epochToXMLTime(int64(c)))
	b.WriteString("</creationdate>")
}

func (g GetLastModifiedPropertyValue) XML(b *bytes.Buffer) {
	b.WriteString("<getlastmodified>")
	b.WriteString(epochToXMLTime(int64(g)))
	b.WriteString("</getlastmodified>")
}

func (l GetContentLengthPropertyValue) XML(b *bytes.Buffer) {
	b.WriteString("<getcontentlength>")
	b.WriteString(fmt.Sprint(l))
	b.WriteString("</getcontentlength>")
}

func (d DisplayNamePropertyValue) XML(b *bytes.Buffer) {
	b.WriteString("<displayname>")
	xml.Escape(b, []byte(d))
	b.WriteString("</displayname>")
}

func (c GetContentTypePropertyValue) XML(b *bytes.Buffer) {
	b.WriteString("<getcontenttype>")
	b.WriteString(string(c))
	b.WriteString("</getcontenttype>")
}

func (c GetEtagPropertyValue) XML(b *bytes.Buffer) {
	b.WriteString("<getetag>")
	b.WriteString(string(c))
	b.WriteString("</getetag>")
}

func (l QuotaAvailableBytesPropertyValue) XML(b *bytes.Buffer) {
	b.WriteString("<quota-available-bytes>")
	b.WriteString(fmt.Sprint(l))
	b.WriteString("</quota-available-bytes>")
}

func (l QuotaUsedBytesPropertyValue) XML(b *bytes.Buffer) {
	b.WriteString("<quota-used-bytes>")
	b.WriteString(fmt.Sprint(l))
	b.WriteString("</quota-used-bytes>")
}

func (r ResourceTypePropertyValue) XML(b *bytes.Buffer) {
	b.WriteString("<resourcetype>")
	if r {
		b.WriteString("<collection/>")
	}
	b.WriteString("</resourcetype>")
}

func epochToXMLTime(sec int64) string {
	return template.HTMLEscapeString(time.Unix(sec, 0).UTC().Format(time.RFC3339))
}
