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

func ListAllZFSDatasets() ([]*zfs.Dataset, error) {
	datasets := []*zfs.Dataset{}

	list, err := zfs.ListZpools()
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Warning("Error reading ZFS datasets.")
		return datasets, err
	}

	for i := range list {
		dataset, _ := list[i].Datasets()
		for a := range dataset {
			datasets = append(datasets, dataset[a])
		}
	}

	return datasets, nil
}

func FindZFSSnapshot(name string) (*zfs.Dataset, error) {
	list, err := ListAllZFSDatasets()
	if err != nil {
		return &zfs.Dataset{}, err
	}

	for d := range list {
			if list[d].Name == name+"@Ready" {
				snapshots, err := list[d].Snapshots()
				if err != nil {
					return &zfs.Dataset{}, err
				}

				for s := range snapshots {
					return snapshots[s], nil
				}
				return &zfs.Dataset{}, fmt.Errorf("Found the dataset but no snapshots.")
			}
	}
	return &zfs.Dataset{}, fmt.Errorf("Failed to find the snapshot.")
}

func SnapshotZFSDataset(dataset zfs.Dataset) (*zfs.Dataset, error) {
	snapshot, err := dataset.Snapshot("Ready", true)
	return snapshot, err
}

func CreateZFSDataset(filesystem string, params map[string]string) (*zfs.Dataset, error) {
	log.WithFields(log.Fields{"dataset": filesystem, "params": params}).Debug("Creating dataset.")
	dataset, err := zfs.CreateFilesystem(filesystem, params)
	return dataset, err
}

func CloneZFSSnapshot(snapshot *zfs.Dataset, destination string, properties map[string]string) (*zfs.Dataset, error) {
	log.WithFields(log.Fields{"snapshot": snapshot.Name, "destination": destination}).Debug("Cloning snapshot to dataset.")

	newDataset, err := snapshot.Clone(destination, properties)

	return newDataset, err
}
