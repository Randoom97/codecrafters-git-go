package commands

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/codecrafters-io/git-starter-go/cmd/mygit/gitobject"
	"github.com/codecrafters-io/git-starter-go/cmd/mygit/readerutils"
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
		reader, err := gitobject.Reader(os.Args[3])
		if err != nil {
			return "", err
		}
		defer reader.Close()

		header := readerutils.ReadToNextNullByte(reader)
		parts := strings.Split(header, " ")
		objectType := parts[0]
		length, _ := strconv.Atoi(parts[1])
		switch objectType {
		case "blob":
			return string(readerutils.ReadNBytes(length, reader)), nil
		case "tree":
			treeNodes := gitobject.ReadTree(length, reader)
			var result strings.Builder
			for _, node := range treeNodes {
				objectType := "blob"
				if node.Mode == 40000 {
					objectType = "tree"
				}
				result.WriteString(fmt.Sprintf("%06d %s %s    %s\n", node.Mode, objectType, node.Hash, node.Name))
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

	reader, err := gitobject.Reader(hash)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	header := readerutils.ReadToNextNullByte(reader)
	parts := strings.Split(header, " ")
	if parts[0] != "tree" {
		return "", fmt.Errorf("%s is not a tree object", hash)
	}
	length, _ := strconv.Atoi(parts[1])

	treeNodes := gitobject.ReadTree(length, reader)
	var result strings.Builder
	for _, node := range treeNodes {
		if nameOnly {
			result.WriteString(fmt.Sprintf("%s\n", node.Name))
		} else {
			objectType := "blob"
			if node.Mode == 40000 {
				objectType = "tree"
			}
			result.WriteString(fmt.Sprintf("%06d %s %s    %s\n", node.Mode, objectType, node.Hash, node.Name))
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
			hash, err = gitobject.WriteObject(blobBytes)
			if err != nil {
				return "", err
			}
		} else {
			hash = gitobject.HashData(blobBytes)
		}

		hashes.WriteString(fmt.Sprintf("%x\n", hash))
	}

	return hashes.String(), nil
}

func WriteTree() (response string, err error) {
	hash, err := gitobject.WriteTree("./")
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

	if objectType, err := gitobject.Type(treeHash); err != nil {
		return "", err
	} else if objectType != "tree" {
		return "", fmt.Errorf("provided hash isn't a tree")
	}
	fullTreeHash, _ := gitobject.FullHash(treeHash)

	var commitByteBuffer bytes.Buffer
	commitByteBuffer.WriteString(fmt.Sprintf("tree %s\n", fullTreeHash))
	if *parentPtr != "" {
		if objectType, err := gitobject.Type(*parentPtr); err != nil {
			return "", err
		} else if objectType != "commit" {
			return "", fmt.Errorf("provided parent isn't a commit")
		}
		fullParentHash, _ := gitobject.FullHash(*parentPtr)

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

	hash, err := gitobject.WriteObject(append(leadingBytes, commitBytes...))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x\n", hash), nil
}
