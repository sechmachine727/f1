package httpapi

import (
	"os"
	"strconv"
)

func createFile(path string) (*os.File, error) { return os.Create(path) }
func itoa(v int64) string                      { return strconv.FormatInt(v, 10) }
