package commands

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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
				objectType := "blob"
				if node.mode == 40000 {
					objectType = "tree"
				}
				result.WriteString(fmt.Sprintf("%06d %s %s    %s\n", node.mode, objectType, node.hash, node.name))
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
			objectType := "blob"
			if node.mode == 40000 {
				objectType = "tree"
			}
			result.WriteString(fmt.Sprintf("%06d %s %s    %s\n", node.mode, objectType, node.hash, node.name))
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

		var hash []byte
		if writeObjects {
			hash, err = writeObject(blobBytes)
			if err != nil {
				return "", err
			}
		} else {
			hash = hashData(blobBytes)
		}

		hashes.WriteString(fmt.Sprintf("%x\n", hash))
	}

	return hashes.String(), nil
}

func WriteTree() (response string, err error) {
	hash, err := writeTree("./")
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x\n", hash), nil
}

func CommitTree() (response string, err error) {
	commitTreeCmd := flag.NewFlagSet("commit-tree", flag.ExitOnError)
	treeHash := os.Args[2]
	parentPtr := commitTreeCmd.String("p", "", "parent commit")
	messagePtr := commitTreeCmd.String("m", "", "commit message")
	commitTreeCmd.Parse(os.Args[3:])

	if *messagePtr == "" {
		return "", fmt.Errorf("commit message can't be empty")
	}

	if objectType, err := gitObjectType(treeHash); err != nil {
		return "", err
	} else if objectType != "tree" {
		return "", fmt.Errorf("provided hash isn't a tree")
	}
	fullTreeHash, _ := fullHash(treeHash)

	var commitByteBuffer bytes.Buffer
	commitByteBuffer.WriteString(fmt.Sprintf("tree %s\n", fullTreeHash))
	if *parentPtr != "" {
		if objectType, err := gitObjectType(*parentPtr); err != nil {
			return "", err
		} else if objectType != "commit" {
			return "", fmt.Errorf("provided parent isn't a commit")
		}
		fullParentHash, _ := fullHash(*parentPtr)

		commitByteBuffer.WriteString(fmt.Sprintf("parent %s\n", fullParentHash))
	}

	currentTime := fmt.Sprint(time.Now().Unix())
	timezone, _ := time.Now().Local().Zone()
	commitByteBuffer.WriteString(fmt.Sprintf("author 123abc <123abc@example.com> %s %s\n", currentTime, timezone))
	commitByteBuffer.WriteString(fmt.Sprintf("committer 123abc <123abc@example.com> %s %s\n", currentTime, timezone))
	commitByteBuffer.WriteString("\n")
	commitByteBuffer.WriteString(fmt.Sprintf("%s\n", *messagePtr))

	commitBytes := commitByteBuffer.Bytes()
	leadingBytes := []byte(fmt.Sprintf("commit %d%c", len(commitBytes), 0))

	hash, err := writeObject(append(leadingBytes, commitBytes...))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x\n", hash), nil
}

func fullHash(hash string) (fullHash string, err error) {
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

	parts := strings.Split(files[0], "/")
	return strings.Join(parts[len(parts)-2:], ""), nil
}

func gitObjectType(hash string) (objectType string, err error) {
	reader, err := gitObjectReader(hash)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	return strings.Split(readHeader(reader), " ")[0], nil
}

func hashData(data []byte) (hash []byte) {
	hasher := sha1.New()
	hasher.Write(data)
	return hasher.Sum(nil)
}

func writeBlob(filepath string) (hash []byte, err error) {
	fileBytes, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	leadingBytes := []byte(fmt.Sprintf("blob %d%c", len(fileBytes), 0))
	return writeObject(append(leadingBytes, fileBytes...))
}

func writeTree(dirpath string) (hash []byte, err error) {
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
			entryHash, err = writeTree(dirpath + "/" + name)
			mode = 40000
		} else {
			entryHash, err = writeBlob(dirpath + "/" + name)
			mode = 100644
		}
		if err != nil {
			return nil, err
		}
		treeByteBuffer.Write(append([]byte(fmt.Sprintf("%d %s%c", mode, name, 0)), entryHash...))
	}
	treeBytes := treeByteBuffer.Bytes()
	leadingBytes := []byte(fmt.Sprintf("tree %d%c", len(treeBytes), 0))

	return writeObject(append(leadingBytes, treeBytes...))
}

func writeObject(data []byte) (hash []byte, err error) {
	hash = hashData(data)

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
