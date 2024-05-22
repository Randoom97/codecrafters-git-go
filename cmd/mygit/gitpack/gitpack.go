package gitpack

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/codecrafters-io/git-starter-go/cmd/mygit/gitobject"
	"github.com/codecrafters-io/git-starter-go/cmd/mygit/readerutils"
)

func Unpack(directory string, reader io.Reader) (err error) {
	packData, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	if string(packData[:4]) != "PACK" {
		return fmt.Errorf("not a valid pack")
	}

	checksum := packData[len(packData)-20:]
	packData = packData[:len(packData)-20]
	if !bytes.Equal(checksum, gitobject.HashData(packData)) {
		return fmt.Errorf("pack data did not pass checksum")
	}

	packBuffer := bytes.NewBuffer(packData)
	readerutils.ReadNBytes(8, packBuffer) // skip 'PACK' and version

	objectCount := binary.BigEndian.Uint32(readerutils.ReadNBytes(4, packBuffer))

	for i := uint32(0); i < objectCount; i++ {
		oType, size := readTypeAndSize(packBuffer)
		switch oType {
		case COMMIT:
			gitobject.WriteCommit(zlibRead(size, packBuffer))
		case TREE:
			gitobject.WriteTree(zlibRead(size, packBuffer))
		case BLOB:
			gitobject.WriteBlob(zlibRead(size, packBuffer))
		case TAG:
			// unsupported
			zlibRead(size, packBuffer)
		case OFS_DELTA:
			return fmt.Errorf("offset deltas aren't currently supported")
		case REF_DELTA:
			referenceHash := readerutils.ReadNBytes(20, packBuffer)
			data := zlibRead(size, packBuffer)
			targetData, err := applyDelta(fmt.Sprintf("%x", referenceHash), data)
			if err != nil {
				return err
			}
			gitobject.WriteObject(targetData)
		}

	}

	return nil
}

func applyDelta(referenceHash string, delta []byte) (targetData []byte, err error) {
	deltaBuffer := bytes.NewBuffer(delta)
	sourceLength := readSize(deltaBuffer)
	targetLength := readSize(deltaBuffer)

	objectType, err := gitobject.Type(referenceHash)
	if err != nil {
		return nil, err
	}
	sourceReader, err := gitobject.Reader(referenceHash)
	if err != nil {
		return nil, err
	}
	defer sourceReader.Close()
	readerutils.ReadToNextNullByte(sourceReader)
	sourceData, err := io.ReadAll(sourceReader)
	if err != nil {
		return nil, err
	}
	if len(sourceData) != int(sourceLength) {
		return nil, fmt.Errorf("source object wasn't the correct length for de deltifying")
	}

	for deltaBuffer.Len() > 0 {
		command := readerutils.ReadByte(deltaBuffer)
		if command&0b10000000 == 0 {
			// insert
			targetData = append(targetData, readerutils.ReadNBytes(int(command&0b1111111), deltaBuffer)...)
		} else {
			// copy
			offset := uint32(0)
			for i := 0; i < 4; i++ {
				if command&(0b1<<i) != 0 {
					offset |= uint32(readerutils.ReadByte(deltaBuffer)) << (8 * i)
				}
			}
			size := uint32(0)
			for i := 0; i < 3; i++ {
				if command&(0b10000<<i) != 0 {
					size |= uint32(readerutils.ReadByte(deltaBuffer)) << (8 * i)
				}
			}

			targetData = append(targetData, sourceData[offset:offset+size]...)
		}
	}

	if len(targetData) != int(targetLength) {
		return nil, fmt.Errorf("target object wasn't the correct length for de deltifying")
	}

	targetData = append([]byte(fmt.Sprintf("%s %d%c", objectType, len(targetData), 0)), targetData...)
	return targetData, nil
}

type objectType uint8

const (
	COMMIT    objectType = 0b001
	TREE      objectType = 0b010
	BLOB      objectType = 0b011
	TAG       objectType = 0100
	OFS_DELTA objectType = 0b110
	REF_DELTA objectType = 0b111
)

func readTypeAndSize(reader io.Reader) (oType objectType, size uint64) {
	// first byte is special because it contains the type
	firstByte := readerutils.ReadByte(reader)
	oType = objectType((firstByte & 0b01110000) >> 4)
	size = uint64(firstByte & 0b1111)

	if firstByte&0b10000000 == 0 {
		return oType, size
	}

	bytesRead := 1
	for {
		b := readerutils.ReadByte(reader)
		bytesRead += 1
		size = size | (uint64(b&0b1111111) << ((bytesRead-2)*7 + 4))
		if b&0b10000000 == 0 {
			break
		}
	}
	return oType, size
}

func readSize(reader io.Reader) (size uint64) {
	size = 0
	bytesRead := 0
	for {
		b := readerutils.ReadByte(reader)
		bytesRead += 1
		size = size | (uint64(b&0b1111111) << ((bytesRead - 1) * 7))
		if b&0b10000000 == 0 {
			break
		}
	}
	return size
}

func zlibRead(size uint64, reader io.Reader) (data []byte) {
	zlibReader, _ := zlib.NewReader(reader)
	data = readerutils.ReadNBytes(int(size), zlibReader)
	zlibReader.Close()
	return data
}
