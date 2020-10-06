package switchfs

import (
	"errors"
	"github.com/avast/retry-go"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
)

type ReadAtCloser interface {
	io.ReaderAt
	io.Closer
}

type splitFile struct {
	info      []os.FileInfo
	files     []ReadAtCloser
	path      string
	chunkSize int64
}

type fileWrapper struct {
	file ReadAtCloser
	path string
}

func NewFileWrapper(filePath string) (*fileWrapper, error) {
	result := fileWrapper{}
	result.path = filePath
	file, err := _openFile(filePath)
	if err != nil {
		return nil, err
	}
	result.file = file
	return &result, nil
}

func (sp *fileWrapper) ReadAt(p []byte, off int64) (n int, err error) {
	if sp.file != nil {
		return sp.file.ReadAt(p, off)
	}
	return 0, errors.New("file is not opened")
}

func (sp *fileWrapper) Close() error {

	if sp.file != nil {
		return sp.file.Close()
	}

	return nil
}

func NewSplitFileReader(filePath string) (*splitFile, error) {
	result := splitFile{}
	index := strings.LastIndex(filePath, string(os.PathSeparator))
	splitFileFolder := filePath[:index]
	files, err := ioutil.ReadDir(splitFileFolder)
	if err != nil {
		return nil, err
	}
	result.path = splitFileFolder
	result.chunkSize = files[0].Size()
	result.info = make([]os.FileInfo, 0, len(files))
	result.files = make([]ReadAtCloser, len(files))
	for _, file := range files {
		if _, err := strconv.Atoi(file.Name()[len(file.Name())-1:]); err == nil {
			result.info = append(result.info, file)
		}
	}
	return &result, nil
}

func (sp *splitFile) ReadAt(p []byte, off int64) (n int, err error) {
	//calculate the part containing the offset
	part := int(off / sp.chunkSize)

	if len(sp.info) < part {
		return 0, errors.New("missing part " + strconv.Itoa(part))
	}

	if len(sp.files) == 0 || sp.files[part] == nil {
		file, _ := _openFile(path.Join(sp.path, sp.info[part].Name()))
		sp.files[part] = file
	}
	off = off - sp.chunkSize*int64(part)

	if off < 0 || off > sp.info[part].Size() {
		return 0, errors.New("offset is out of bounds")
	}
	return sp.files[part].ReadAt(p, off)
}

func _openFile(path string) (*os.File, error) {
	var file *os.File
	var err error
	retry.Attempts(5)
	err = retry.Do(
		func() error {
			file, err = os.Open(path)
			return err
		},
	)
	return file, err
}

func (sp *splitFile) Close() error {
	for _, file := range sp.files {
		if file != nil {
			file.Close()
		}
	}
	return nil
}

func OpenFile(filePath string) (ReadAtCloser, error) {
	//check if it's a split file
	if _, err := strconv.Atoi(filePath[len(filePath)-1:]); err == nil {
		return NewSplitFileReader(filePath)
	} else {
		return NewFileWrapper(filePath)
	}
}
