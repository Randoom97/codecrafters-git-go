package gitobject

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/codecrafters-io/git-starter-go/cmd/mygit/readerutils"
)

func FullHash(partialHash string) (fullHash string, err error) {
	file, err := fileForHash(partialHash)
	if err != nil {
		return "", err
	}

	parts := strings.Split(file, "/")
	return strings.Join(parts[len(parts)-2:], ""), nil
}

func Type(hash string) (objectType string, err error) {
	reader, err := Reader(hash)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	return strings.Split(readerutils.ReadToNextNullByte(reader), " ")[0], nil
}

func HashData(data []byte) (hash []byte) {
	hasher := sha1.New()
	hasher.Write(data)
	return hasher.Sum(nil)
}

func WriteBlobFromFile(filepath string) (hash []byte, err error) {
	fileBytes, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	return WriteBlob(fileBytes)
}

func WriteBlob(data []byte) (hash []byte, err error) {
	leadingBytes := []byte(fmt.Sprintf("blob %d%c", len(data), 0))
	return WriteObject(append(leadingBytes, data...))
}

func WriteTreeFromDirectory(dirpath string) (hash []byte, err error) {
	dirEntries, err := os.ReadDir(dirpath)
	if err != nil {
		return nil, err
	}

	var treeByteBuffer bytes.Buffer
	for _, dirEntry := range dirEntries {
		name := dirEntry.Name()
		if name == ".git" {
			continue
		}
		var entryHash []byte
		var mode int
		if dirEntry.IsDir() {
			entryHash, err = WriteTreeFromDirectory(dirpath + "/" + name)
			mode = 40000
		} else {
			entryHash, err = WriteBlobFromFile(dirpath + "/" + name)
			mode = 100644
		}
		if err != nil {
			return nil, err
		}
		treeByteBuffer.Write(append([]byte(fmt.Sprintf("%d %s%c", mode, name, 0)), entryHash...))
	}
	treeBytes := treeByteBuffer.Bytes()
	return WriteTree(treeBytes)
}

func WriteTree(data []byte) (hash []byte, err error) {
	leadingBytes := []byte(fmt.Sprintf("tree %d%c", len(data), 0))
	return WriteObject(append(leadingBytes, data...))
}

func WriteCommit(data []byte) (hash []byte, err error) {
	leadingBytes := []byte(fmt.Sprintf("commit %d%c", len(data), 0))
	return WriteObject(append(leadingBytes, data...))
}

func WriteObject(data []byte) (hash []byte, err error) {
	hash = HashData(data)

	directory := fmt.Sprintf(".git/objects/%x", hash[:1])
	if err := os.MkdirAll(directory, 0755); err != nil {
		return nil, fmt.Errorf("error creating directory: %s", err)
	}

	filepath := fmt.Sprintf("%s/%x", directory, hash[1:])
	file, err := os.Create(filepath)
	if err != nil {
		return nil, fmt.Errorf("error creating file: %s", err)
	}
	defer file.Close()

	w := zlib.NewWriter(file)
	_, err = w.Write(data)
	if err != nil {
		return nil, fmt.Errorf("error writing file: %s", err)
	}
	w.Close()

	return hash, nil
}

type TreeNode struct {
	Mode int
	Name string
	Hash string
}

func ReadTree(length int, reader io.Reader) []TreeNode {
	result := []TreeNode{}
	for length > 0 {
		header := readerutils.ReadToNextNullByte(reader)
		parts := strings.Split(header, " ")

		mode, _ := strconv.Atoi(parts[0])
		name := parts[1]
		hash := fmt.Sprintf("%x", readerutils.ReadNBytes(20, reader))
		result = append(result, TreeNode{mode, name, hash})

		length -= len(header) + 21
	}
	return result
}

func Reader(hash string) (reader io.ReadCloser, err error) {
	filepath, err := fileForHash(hash)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}

	return zlib.NewReader(file)
}

func fileForHash(hash string) (file string, err error) {
	if len(hash) < 2 {
		return "", fmt.Errorf("provided hash isn't long enough")
	}
	directory := hash[:2]
	filename := hash[2:]

	files, err := filepath.Glob(fmt.Sprintf(".git/objects/%s/%s*", directory, filename))
	if err != nil {
		return "", err
	}

	if len(files) < 1 {
		return "", fmt.Errorf("fatal: Not a valid object name %s", hash)
	}
	if len(files) > 1 {
		return "", fmt.Errorf("provided hash isn't unique enough")
	}

	return files[0], nil
}
