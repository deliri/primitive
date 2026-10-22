//go:build !darwin

package filestore

import "os"

func syncTreeMember(file *os.File) error { return file.Sync() }
