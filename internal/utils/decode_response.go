package utils

import (
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"
)

func DecodeResponse(response *http.Response) ([]byte, error) {
	var body io.ReadCloser

	switch response.Header.Get("Content-Encoding") {
	case "gzip":
		var err error
		body, err = gzip.NewReader(response.Body)
		if err != nil {
			body = response.Body
		}
	default:
		body = response.Body
	}

	bodyByte, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}

	lineBreak := []byte("\n")
	bodyByte = append(bodyByte, lineBreak...)
	bodyByte = bodyByte[0:500]

	defer body.Close()

	return bodyByte, nil
}