package main

import (
	"encoding/json"
	"github.com/mistifyio/go-zfs"
	"net/http"
	"errors"
	"fmt"
	"regexp"
)

type InitResponse struct {
	Message string
	Error   error
	Datasets []zfs.Dataset
}

type InitCreate struct {
	ZFSParams ZFSParams
	FreeBSDParams FreeBSDParams

}

type ZFSParams struct {
	BaseDataset string
	Mountpoint  string
	Compression bool
}

type FreeBSDParams struct {
	Version string
}

func initDataset(i InitCreate) ([]zfs.Dataset, error) {
	var datasets []zfs.Dataset

	rootOpts := make(map[string]string)
	rootOpts["mountpoint"] = i.ZFSParams.Mountpoint
	if i.ZFSParams.Compression {
		rootOpts["compression"] = "on"
	}
	rootJailDataset, err := zfs.CreateFilesystem(i.ZFSParams.BaseDataset, rootOpts)
	if err != nil {
		return datasets, err
	}
	rootJailDataset.SetProperty("jest:name", "root")

	baseOpts := make(map[string]string)
	baseOpts["mountpoint"] = i.ZFSParams.Mountpoint + "/"
	baseJailDataset, err := zfs.CreateFilesystem(i.ZFSParams.BaseDataset + "/." + i.FreeBSDParams.Version, baseOpts)
	if err != nil {
		return datasets, err
	}
	baseJailDataset.SetProperty("jest:name", "baseJail")

	datasets = append(datasets, *rootJailDataset, *baseJailDataset)
	return datasets, nil
}

func downloadVersion(f FreeBSDParams) {

}

func validateVersion(v string) error {
	r, err := regexp.Compile(`^[0-9]*\.[0-9]*-[A-Z0-9]*$`)
	if err != nil {
		return err
	}

	if r.MatchString(v) == false {
		//toDo: Find out why this error doesn't get returned in our response
		return errors.New("The version specified: " + v + " is not valid. The version should match the regex ^[0-9]*.[0-9]*-[A-Z0-9]*$")
	}

	return nil
}

func CreateInitEndpoint(w http.ResponseWriter, r *http.Request) {
	var i InitCreate
	var datasets []zfs.Dataset

	err := json.NewDecoder(r.Body).Decode(&i)
	if err != nil {
		w.WriteHeader(http.StatusNotAcceptable)
		json.NewEncoder(w).Encode(InitResponse{"Failed to parse the JSON request.", err,datasets})
		return
	}

	err = validateVersion(i.FreeBSDParams.Version)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(InitResponse{"Invalid FreeBSD Version specified.", err,datasets})
		return
	}

	datasets, err = initDataset(i)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(InitResponse{"Failed to create the dataset " + i.ZFSParams.BaseDataset + ".", err, datasets})
		return
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(InitResponse{"Successfully initialised dataset " + i.ZFSParams.BaseDataset + ".", nil, datasets})
}

func GetInitEndpoint(w http.ResponseWriter, r *http.Request) {
	var datasets []zfs.Dataset

	l, err := zfs.ListZpools()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(InitResponse{"Failed to list list Zpools on the system.", err, datasets})
		return
	}

	for i := range l {
		d, _ := l[i].Datasets()
		for a := range d {
			jestName, _ := d[a].GetProperty("jest:name")
			fmt.Println(jestName)
			if jestName != "-" {
				datasets = append(datasets, *d[a])
			}
		}
	}

	if len(datasets) == 0 {
		w.WriteHeader(http.StatusNoContent)
		json.NewEncoder(w).Encode(InitResponse{"Failed to find any ZFS datasets registered with Jest.", errors.New("No ZFS datasets containing property jest:name found"), datasets})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(InitResponse{"Successfully found Jest datasets.", nil, datasets})
}

func DeleteInitEndpoint(w http.ResponseWriter, r *http.Request) {

}
