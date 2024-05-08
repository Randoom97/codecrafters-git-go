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
		hash := os.Args[3]
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

		file, err := os.Open(files[0])
		if err != nil {
			return "", err
		}
		reader, err := zlib.NewReader(file)
		if err != nil {
			return "", err
		}
		defer reader.Close()

		blobData, err := io.ReadAll(reader)
		if err != nil {
			return "", err
		}

		spaceIndex := bytes.IndexByte(blobData, ' ')
		switch objectType := string(blobData[:spaceIndex]); objectType {
		case "blob":
			nullIndex := bytes.IndexByte(blobData, 0)
			length, err := strconv.Atoi(string(blobData[spaceIndex+1 : nullIndex]))
			if err != nil {
				return "", err
			}
			return string(blobData[nullIndex+1 : nullIndex+length+1]), nil

		default:
			return "", fmt.Errorf("unsupported git object type: %s", objectType)
		}

	default:
		return "", fmt.Errorf("unknown arguments for cat-file")
	}
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
