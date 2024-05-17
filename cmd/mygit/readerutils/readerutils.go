package readerutils

import "io"

func ReadToNextNullByte(reader io.Reader) (header string) {
	bytes := []byte{}
	for {
		data := ReadNBytes(1, reader)
		if len(data) < 1 || data[0] == 0 {
			break
		}
		bytes = append(bytes, data[0])
	}
	return string(bytes)
}

func ReadNBytes(n int, reader io.Reader) (data []byte) {
	blobData := make([]byte, n)
	io.ReadFull(reader, blobData)
	return blobData
}
