package gdrive

import (
	log "github.com/cihub/seelog"
	"io/ioutil"
	"net/http"
	"bytes"
)

type loggingTransport struct {
	rt http.RoundTripper
}

func (t *loggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.rt.RoundTrip(req)

	if resp.StatusCode >= 500 {
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
		  return nil, err
		}
		log.Error("5xx error from server:", resp, "\nBody:\n", string(body))
		resp.Body = ioutil.NopCloser(bytes.NewBuffer(body))
	}

	return resp, err
}
