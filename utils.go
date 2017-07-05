package main

import (
	"os"
	"path/filepath"
	log "github.com/sirupsen/logrus"
	"github.com/mholt/archiver"
	"io"
	"strings"
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


func GetEnv() map[string]string {
	getEnvironment := func(data []string, getKeyVal func(item string) (key, val string)) map[string]string {
		items := make(map[string]string)
		for _, item := range data {
			key, val := getKeyVal(item)
			items[key] = val
		}
		return items
	}
	environment := getEnvironment(os.Environ(), func(item string) (key, val string) {
		splits := strings.Split(item, "=")
		key = splits[0]
		val = splits[1]
		return
	})
	return environment
}

func SetEnv(envs map[string]string) {
	for key, value := range envs {
		os.Setenv(key, value)
	}
}