package grpcweb

import (
	"bytes"
	"encoding/binary"
	"io"
	"io/ioutil"
)

func NewJsonReader(body io.ReadCloser) (io.ReadCloser, error) {
	content, err := ioutil.ReadAll(body)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	newBody := make([]byte, len(content)+5)
	binary.BigEndian.PutUint32(newBody[1:], uint32(len(content)))
	copy(newBody[5:], content)
	return &jsonReader{ioutil.NopCloser(bytes.NewBuffer(newBody))}, nil
}

type jsonReader struct {
	io.ReadCloser
}
