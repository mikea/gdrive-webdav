package webdav

import (
	"bytes"
	"encoding/xml"
	"fmt"
	log "github.com/cihub/seelog"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
)

type handler struct {
	fs FileSystem
}

func (h *handler) handle(w http.ResponseWriter, r *http.Request) {
	log.Info(r.Method, " ", *r)

	switch r.Method {
	case "GET":
		status, response := h.handleGet(w, r)
		writeStatusAndResponse(status, response, w)
	case "HEAD":
		status, response := h.handleGet(w, r)
		if response != nil {
			defer response.Close()
		}
		writeStatus(status, w)
	case "MKCOL":
		writeStatus(h.handleMkcol(r), w)
	case "DELETE":
		writeStatus(h.handleDelete(r), w)
	case "OPTIONS":
		w.Header().Set("DAV", "1" /*, 2" */)
	case "PROPFIND":
		status, response := h.handlePropfind(r)
		writeStatusAndResponse(status, response, w)
	case "PUT":
		writeStatus(h.handlePut(r), w)
	case "COPY":
		writeStatus(h.handleCopy(r), w)
  case "MOVE":
    writeStatus(h.handleMove(r), w)
	case "LOCK":
		writeStatus(h.handleLock(r), w)
	default:
		log.Errorf("method not supported")
		w.WriteHeader(http.StatusBadRequest)
	}
}

func writeStatus(status StatusCode, w http.ResponseWriter) {
	log.Debug("STATUS ", int(status))
	w.WriteHeader(int(status))
}

func writeStatusAndResponse(status StatusCode, r io.ReadCloser, w http.ResponseWriter) {
	writeStatus(status, w)

	if r != nil {
		defer r.Close()
		written, err := io.Copy(w, r)
		if err != nil {
			log.Errorf("Can't write response: %v", err)
		} else {
			log.Debug("Copied ", written, " bytes")
		}
	}
}

func (h *handler) handleMkcol(r *http.Request) StatusCode {
	if r.ContentLength > 0 {
		log.Errorf("ERROR: content length > 0")
		return StatusCode(415)
	}

	p := url2path(r.URL)

	return StatusCode(h.fs.MkDir(p))
}

func (h *handler) handleDelete(r *http.Request) StatusCode {
	p := url2path(r.URL)
	return StatusCode(h.fs.Delete(p))
}

func (h *handler) handlePut(r *http.Request) StatusCode {
	p := url2path(r.URL)
	return h.fs.Put(p, r.Body)
}

func (h *handler) handleLock(r *http.Request) StatusCode {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error("can't read from body ", err)
		return StatusCode(500)
	}

	log.Debug(" body: ", string(body))

	return StatusCode(204)
}

func (h *handler) handleGet(w http.ResponseWriter, r *http.Request) (StatusCode, io.ReadCloser) {
	p := url2path(r.URL)
	status, stream, length := h.fs.Get(p)

	if int(status) != 200 {
		return status, nil
	}

	log.Debug("got length ", length)
	w.Header().Set("Content-length", fmt.Sprintf("%d", length))
	w.Header().Set("Content-Type", "application/octet-stream")

	return StatusCode(200), stream
}

func (h *handler) handlePropfind(r *http.Request) (status StatusCode, reader io.ReadCloser) {
	// xml parsing error
	defer func() {
		if err, ok := recover().(XmlError); ok {
			log.Error("Can't parse xml. error=", err)
			status = StatusCode(400)
		}
	}()

	depth := r.Header.Get("Depth")

	if depth != "0" && depth != "1" {
		log.Error("Unsupported depth ", depth)
		return StatusCode(500), nil
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error("can't read from body ", err)
		return StatusCode(500), nil
	}

	log.Debug("Depth: ", depth, " body: ", string(body))

	x := parsexml(bytes.NewReader(body))
	x.mustStart("propfind")

	var propsToFind []string

	switch {
	case x.start("prop"):
		propsToFind = parseProp(x)
		x.mustEnd("prop")
	case x.start("allprop"):
		propsToFind = []string{"getcontentlength", "displayname", "resourcetype", "getcontenttype", "getlastmodified", "creationdate"}
		x.mustEnd("allprop")
	default:
		log.Error("Unsupported element")
		return StatusCode(500), nil
	}

	p := url2path(r.URL)
	log.Debug("p=", p, " PropsToFind=", propsToFind)
	d, err := strconv.Atoi(depth)

	if err != nil {
		log.Error("Can't parse depth", depth)
		return StatusCode(500), nil
	}

	status, result := h.fs.PropList(p, d, propsToFind)

	if result == nil {
		log.Debug("Result is nil: ", status)
		return status, nil
	}

	log.Debug("result=", result)

	var ms multistatus

	for path, values := range result {
		resp := &response{href: href(path)}

		// Sadly slices are not polymorphic
		var props []xmlSerializable
		for _, v := range values {
			props = append(props, v)
		}
		resp.body = propstats{{props, 200}}
		ms = append(ms, resp)
	}

	xmlbytes := new(bytes.Buffer)
	ms.XML(xmlbytes)
	//	w.Header().Set("Content-Type", "application/xml; charset=UTF-8")

	return StatusCode(207), ioutil.NopCloser(xmlbytes)
}

func (h *handler) handleCopy(r *http.Request) StatusCode {
	var err error

	p := url2path(r.URL)

	strDepth := r.Header.Get("Depth")
	depth := 9999999
	if strDepth != "" && strDepth != "infinity" {
		depth, err = strconv.Atoi(strDepth)
		if err != nil {
			log.Error("Can't parse depth ", err)
			return StatusCode(500)
		}
	}

	dest, err := urlstring2path(r.Header.Get("Destination"))
	if err != nil {
		log.Error("Can't parse dest ", err)
		return StatusCode(500)
	}

	overwrite := true

	if r.Header.Get("Overwrite") == "F" {
		overwrite = false
	}

	log.Debug("Copy from ", p, " to ", dest, " depth=", depth, " overwrite=", overwrite)

	return StatusCode(h.fs.Copy(p, dest, depth, overwrite))
}


func (h *handler) handleMove(r *http.Request) StatusCode {
  var err error

  p := url2path(r.URL)

  dest, err := urlstring2path(r.Header.Get("Destination"))
  if err != nil {
    log.Error("Can't parse dest ", err)
    return StatusCode(500)
  }

  overwrite := true

  if r.Header.Get("Overwrite") == "F" {
    overwrite = false
  }

  log.Debug("Copy from ", p, " to ", dest, " overwrite=", overwrite)

  return StatusCode(h.fs.Move(p, dest, overwrite))
}

func url2path(url_ *url.URL) string {
	return url_.Path
}

func urlstring2path(u string) (string, error) {
	parsed, err := url.Parse(u)
	if err != nil {
		return "", err
	}
	return parsed.Path, nil
}

func parseProp(x *xmlparser) (props []string) {
	for {
		el, ok := x.cur.(xml.StartElement)
		if !ok {
			break
		}
		props = append(props, el.Name.Local)
		x.mustStart(el.Name.Local)
		x.mustEnd(el.Name.Local)
	}
	return
}
