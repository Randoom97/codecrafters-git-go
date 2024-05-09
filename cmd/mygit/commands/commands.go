package commands

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
)

func Initialize() (response string, err error) {
	for _, dir := range []string{".git", ".git/objects", ".git/refs"} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("error creating directory: %s", err)
		}
	}

	headFileContents := []byte("ref: refs/heads/main\n")
	if err := os.WriteFile(".git/HEAD", headFileContents, 0644); err != nil {
		return "", fmt.Errorf("error writing file: %s", err)
	}

	return "Initialized git directory\n", nil
}

func CatFile() (response string, err error) {
	if len(os.Args) < 4 {
		return "", fmt.Errorf("usage: mygit cat-file <type> <object>")
	}
	switch argType := os.Args[2]; argType {
	case "-p":
		reader, err := gitObjectReader(os.Args[3])
		if err != nil {
			return "", err
		}
		defer reader.Close()

		header := readHeader(reader)
		parts := strings.Split(header, " ")
		objectType := parts[0]
		length, _ := strconv.Atoi(parts[1])
		switch objectType {
		case "blob":
			return string(readNBytes(length, reader)), nil
		case "tree":
			treeNodes := readTree(length, reader)
			var result strings.Builder
			for _, node := range treeNodes {
				result.WriteString(fmt.Sprintf("%d %s %s\n", node.mode, node.name, node.hash))
			}
			return result.String(), nil
		default:
			return "", fmt.Errorf("unsupported git object type: %s", objectType)
		}

	default:
		return "", fmt.Errorf("unknown arguments for cat-file")
	}
}

func LsTree() (response string, err error) {
	var hash string
	nameOnly := false

	switch os.Args[2] {
	case "--name-only":
		nameOnly = true
		hash = os.Args[3]
	default:
		hash = os.Args[2]
	}

	reader, err := gitObjectReader(hash)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	header := readHeader(reader)
	parts := strings.Split(header, " ")
	if parts[0] != "tree" {
		return "", fmt.Errorf("%s is not a tree object", hash)
	}
	length, _ := strconv.Atoi(parts[1])

	treeNodes := readTree(length, reader)
	var result strings.Builder
	for _, node := range treeNodes {
		if nameOnly {
			result.WriteString(fmt.Sprintf("%s\n", node.name))
		} else {
			result.WriteString(fmt.Sprintf("%d %s %s\n", node.mode, node.name, node.hash))
		}
	}
	return result.String(), nil

}

func HashObject() (response string, err error) {
	var pattern string
	writeObjects := false

	switch os.Args[2] {
	case "-w":
		writeObjects = true
		pattern = os.Args[3]
	default:
		pattern = os.Args[2]
	}
	paths, err := filepath.Glob(pattern)

	if err != nil {
		return "", err
	}
	if len(paths) < 1 {
		return "", fmt.Errorf("no files found with pattern: %s", pattern)
	}

	var hashes strings.Builder
	for _, path := range paths {
		fileBytes, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		leadingBytes := []byte(fmt.Sprintf("blob %d%c", len(fileBytes), 0))
		blobBytes := append(leadingBytes, fileBytes...)

		hasher := sha1.New()
		hasher.Write(blobBytes)
		hash := fmt.Sprintf("%x", hasher.Sum(nil))
		hashes.WriteString(hash + "\n")

		if !writeObjects {
			continue
		}

		var compressedBytes bytes.Buffer
		w := zlib.NewWriter(&compressedBytes)
		w.Write(blobBytes)
		compressedBytes.Bytes()
		w.Close()

		directory := fmt.Sprintf(".git/objects/%s", hash[:2])
		if err := os.MkdirAll(directory, 0755); err != nil {
			return "", fmt.Errorf("error creating directory: %s", err)
		}
		filepath := fmt.Sprintf("%s/%s", directory, hash[2:])
		if err := os.WriteFile(filepath, compressedBytes.Bytes(), 0644); err != nil {
			return "", fmt.Errorf("error writing file: %s", err)
		}
	}

	return hashes.String(), nil
}

func readNBytes(n int, reader io.Reader) (data []byte) {
	blobData := make([]byte, n)
	io.ReadFull(reader, blobData)
	return blobData
}

type treeNode struct {
	mode int
	name string
	hash string
}

func readTree(length int, reader io.Reader) []treeNode {
	result := []treeNode{}
	for length > 0 {
		header := readHeader(reader)
		parts := strings.Split(header, " ")

		mode, _ := strconv.Atoi(parts[0])
		name := parts[1]
		hash := fmt.Sprintf("%x", readNBytes(20, reader))
		result = append(result, treeNode{mode, name, hash})

		length -= len(header) + 21
	}
	return result
}

func gitObjectReader(hash string) (reader io.ReadCloser, err error) {
	if len(hash) < 2 {
		return nil, fmt.Errorf("provided hash isn't long enough")
	}
	directory := hash[:2]
	filename := hash[2:]

	files, err := filepath.Glob(fmt.Sprintf(".git/objects/%s/%s*", directory, filename))
	if err != nil {
		return nil, err
	}

	if len(files) < 1 {
		return nil, fmt.Errorf("fatal: Not a valid object name %s", hash)
	}
	if len(files) > 1 {
		return nil, fmt.Errorf("provided hash isn't unique enough")
	}

	file, err := os.Open(files[0])
	if err != nil {
		return nil, err
	}
	return zlib.NewReader(file)
}

func readHeader(reader io.Reader) (header string) {
	bytes := []byte{}
	for {
		data := readNBytes(1, reader)
		if len(data) < 1 || data[0] == 0 {
			break
		}
		bytes = append(bytes, data[0])
	}
	return string(bytes)
}
