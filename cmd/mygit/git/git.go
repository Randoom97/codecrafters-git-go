package git

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/codecrafters-io/git-starter-go/cmd/mygit/gitobject"
	"github.com/codecrafters-io/git-starter-go/cmd/mygit/readerutils"
)

func Initialize(createMainBranch bool) (err error) {
	for _, dir := range []string{".git", ".git/objects", ".git/refs"} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("error creating directory: %s", err)
		}
	}

	if createMainBranch {
		headFileContents := []byte("ref: refs/heads/main\n")
		if err := os.WriteFile(".git/HEAD", headFileContents, 0644); err != nil {
			return fmt.Errorf("error writing file: %s", err)
		}
	}
	return nil
}

func MakeBranch(ref string, hash string) (err error) {
	objectType, err := gitobject.Type(hash)
	if err != nil {
		return err
	}
	if objectType != "commit" {
		return fmt.Errorf("%s isn't a commit and so can't be made a branch", hash)
	}

	if err := os.MkdirAll(".git/refs/heads", 0755); err != nil {
		return err
	}

	if err := os.WriteFile(fmt.Sprintf(".git/refs/heads/%s", ref), []byte(hash+"\n"), 0644); err != nil {
		return err
	}

	return nil
}

func Checkout(ref string) (err error) {
	hash, err := os.ReadFile(fmt.Sprintf(".git/refs/heads/%s", ref))
	if err != nil {
		return err
	}
	stringHash := string(hash[:len(hash)-1]) // remove trailing \n

	headFileContents := []byte(fmt.Sprintf("ref: refs/heads/%s\n", ref))
	if err := os.WriteFile(".git/HEAD", headFileContents, 0644); err != nil {
		return fmt.Errorf("error writing file: %s", err)
	}

	commitReader, err := gitobject.Reader(stringHash)
	if err != nil {
		return err
	}
	readerutils.ReadToNextNullByte(commitReader)
	readerutils.ReadNBytes(5, commitReader) // 'tree '
	treeHash := string(readerutils.ReadNBytes(20, commitReader))
	commitReader.Close()
	return constructTree("./", treeHash)
}

func constructTree(path string, hash string) (err error) {
	treeReader, err := gitobject.Reader(hash)
	if err != nil {
		return err
	}
	defer treeReader.Close()

	length, err := strconv.Atoi(strings.Split(string(readerutils.ReadToNextNullByte(treeReader)), " ")[1])
	if err != nil {
		return err
	}
	treeNodes := gitobject.ReadTree(length, treeReader)

	for _, treeNode := range treeNodes {
		if treeNode.Mode == 40000 {
			if err = os.Mkdir(path+treeNode.Name, 0755); err != nil {
				return err
			}
			if err = constructTree(path+treeNode.Name+"/", treeNode.Hash); err != nil {
				return err
			}
		} else {
			if err = constructBlob(path, treeNode.Name, treeNode.Hash); err != nil {
				return err
			}
		}
	}

	return nil
}

func constructBlob(path string, name string, hash string) (err error) {
	blobReader, err := gitobject.Reader(hash)
	if err != nil {
		return err
	}
	readerutils.ReadToNextNullByte(blobReader)
	blobData, err := io.ReadAll(blobReader)
	if err != nil {
		return err
	}
	return os.WriteFile(path+name, blobData, 0644)
}
