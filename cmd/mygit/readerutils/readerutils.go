package readerutils

import (
	"io"
	"strconv"
)

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

func ReadByte(reader io.Reader) byte {
	return ReadNBytes(1, reader)[0]
}

func ReadGitPackLine(reader io.Reader) (data []byte) {
	length, _ := strconv.ParseUint(string(ReadNBytes(4, reader)), 16, 32)
	if length <= 4 {
		return nil
	}
	return ReadNBytes(int(length)-4, reader)
}
