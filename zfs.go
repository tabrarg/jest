package main

import (
	"github.com/mistifyio/go-zfs"
	"runtime"
	"sync"
)

var wg sync.WaitGroup

//ToDo: Add error handling
func setZFSProperty(dataset *zfs.Dataset, propChan *chan map[string]string) {
	property := <-*propChan

	for key, val := range property {
		dataset.SetProperty(key, val)
	}

	wg.Done()
}

/*
	Concurrently set custom properties on a ZFS dataset
*/
// ToDo: add error handling
func SetZFSProperties(dataset *zfs.Dataset, properties map[string]string) {
	propChan := make(chan map[string]string)
	workers := runtime.NumCPU() * 4
	wg.Add(workers)

	go func() {
		for key, val := range properties {
			propChan <- map[string]string{key: val}
		}
	}()

	for i := 0; i < workers; i++ {
		go setZFSProperty(dataset, &propChan)
	}

	wg.Wait()
}

type propertyMessage struct {
	property map[string]string
	err      error
}

func getZFSProperty(dataset *zfs.Dataset, propChanIn *chan string, propChanOut *chan propertyMessage) {
	key := <-*propChanIn
	val, err := dataset.GetProperty(key)
	*propChanOut <- propertyMessage{map[string]string{key: val}, err}

	wg.Done()
}

func GetZFSProperties(dataset zfs.Dataset, properties []string) (map[string]string, error) {
	propChanIn := make(chan string)
	propChanOut := make(chan propertyMessage)
	propertiesOut := make(map[string]string)

	workers := runtime.NumCPU() * 4
	wg.Add(workers)

	go func() {
		for i := range properties {
			propChanIn <- properties[i]
		}
	}()

	for i := 0; i < workers; i++ {
		go getZFSProperty(&dataset, &propChanIn, &propChanOut)
	}

	wg.Wait()

	for i := range propChanOut {
		if i.err == nil {
			for key, val := range i.property {
				propertiesOut[key] = val
			}
		}
	}

	return propertiesOut, nil
}
