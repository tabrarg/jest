package main

import (
	"fmt"
	"github.com/mistifyio/go-zfs"
	log "github.com/sirupsen/logrus"
)

func SearchZFSProperties(property string) (string, error) {
	log.Debug("Looking for ZFS datasets with the property " + property + " set.")
	list, err := zfs.ListZpools()
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Warning("Error reading ZFS datasets.")
		return "", err
	}

	for i := range list {
		d, _ := list[i].Datasets()
		for a := range d {
			zfsProperty, _ := d[a].GetProperty(property)
			if zfsProperty != "" {
				if zfsProperty != "-" {
					return zfsProperty, nil
				}
			}
		}
	}

	return "", fmt.Errorf("Couldn't find any ZFS datasets with the property " + property + " - please initialise Jest.")
}

func SnapshotZFSDataset(dataset zfs.Dataset) (*zfs.Dataset, error) {
	snapshot, err := dataset.Snapshot("Ready", true)
	return snapshot, err
}