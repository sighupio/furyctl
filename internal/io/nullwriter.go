package io

func NewNullWriter() *NullWriter {
	return &NullWriter{}
}

type NullWriter struct {
}

func (nw *NullWriter) Write(p []byte) (n int, err error) {
	return 0, nil
}
