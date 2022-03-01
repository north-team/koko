package recorderstorage

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/jumpserver/koko/pkg/jms-sdk-go/model"
	"github.com/jumpserver/koko/pkg/jms-sdk-go/service"
)

type ServerStorage struct {
	StorageType string
	FileType    string
	JmsService  *service.JMService
}

func (s ServerStorage) BulkSave(commands []*model.Command) (err error) {
	return s.JmsService.PushSessionCommand(commands)
}

func (s ServerStorage) Upload(gZipFilePath, target string) (err error) {
	id := strings.Split(filepath.Base(gZipFilePath), ".")[0]
	switch s.FileType {
	case "replay":
		return s.JmsService.Upload(id, gZipFilePath)
	case "file":
		return s.JmsService.PushFTPLogFile(id, gZipFilePath)
	default:
		return errors.New("cannot match FileType of ServerStorage")
	}
}

func (s ServerStorage) TypeName() string {
	return s.StorageType
}
