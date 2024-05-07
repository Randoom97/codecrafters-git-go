package commands

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
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
