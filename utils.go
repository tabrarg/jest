package main

import (
	"github.com/mholt/archiver"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

/*
	Append a string to the end of a file.
	Remember to put a newline '\n' at the of the string.
*/
func AppendStringToFile(path, text string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(text)
	if err != nil {
		return err
	}
	return nil
}

func RandomString(strlen int) string {
	r = rand.New(rand.NewSource(time.Now().UnixNano()))
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	result := make([]byte, strlen)
	for i := range result {
		result[i] = chars[r.Intn(len(chars))]
	}
	return string(result)
}

func extractFile(path string, file string, errors chan error) chan error {
	log.WithFields(log.Fields{"file": file, "path": path}).Debug("Extracting archive file.")
	err := archiver.TarXZ.Open(filepath.Join(path, file), path)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "file": file, "path": path}).Warning("Couldn't extract the archive file.")
		errors <- err
	} else {
		log.WithFields(log.Fields{"file": file, "path": path}).Debug("Extracted archive file.")
		errors <- nil
	}
	return errors
}

/*
	Concurrently extract .xv files (used on the FreeBSD FTP site).
*/
func ExtractFiles(path string, files []string) error {
	errors := make(chan error, len(files))

	for i := 0; i < len(files); i++ {
		go extractFile(path, files[i], errors)
	}

	for i := 0; i < len(files); i++ {
		err := <-errors
		if err != nil {
			return err
		}
	}
	return nil
}

// Copy a file - provide a source and destination string.
func CopyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return
	}
	defer func() {
		cErr := out.Close()
		if err == nil {
			err = cErr
		}
	}()
	if _, err = io.Copy(out, in); err != nil {
		return
	}
	err = out.Sync()
	return
}

func Chroot(path string) (func() error, error) {
	root, err := os.Open("/")
	if err != nil {
		return nil, err
	}

	if err := syscall.Chroot(path); err != nil {
		root.Close()
		return nil, err
	}

	return func() error {
		defer root.Close()
		if err := root.Chdir(); err != nil {
			return err
		}
		return syscall.Chroot(".")
	}, nil
}

func CheckFileForString(file string, str string) (bool, error) {
	f, err := ioutil.ReadFile(file)

	if err != nil {
		log.WithFields(log.Fields{"fileName": "/etc/rc.conf", "error": err}).Warning("/etc/rc.conf doesn't exist!")
		return false, err
	}

	s := string(f)
	if strings.Contains(s, str) == true {
		log.WithFields(log.Fields{"fileName": file}).Debug(str + " already exists in " + file)
		return true, nil
	} else {
		log.WithFields(log.Fields{"fileName": file}).Debug(str + " not found in " + file)
		return false, nil
	}
}
