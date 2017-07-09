package main

import (
	"github.com/boltdb/bolt"
	"bytes"
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"fmt"
)

type Config struct {
	JestDir     string // The directory path for Jest
	JestDataset string // The name of the ZFS dataset for Jest (usually mounted on /usr/jail)
	Disabled    bool
}

func LoadConfig() (Config, error) {
	var config = Config{}
	var validConfig = Config{}

	JestDB.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("config"))

		c := b.Cursor()

		for k, v := c.First(); k != nil; k, v = c.Next() {
			encoded := bytes.NewReader(v)
			err := json.NewDecoder(encoded).Decode(&config)
			if err != nil {
				log.Warn("Couldn't decode a key:", err)
			}

			if config.Disabled == false {
				validConfig = config
			}
		}
		return nil
	})

	if validConfig.Disabled == false {
		return validConfig, nil
	}

	return validConfig, fmt.Errorf("Failed to load the config file")
}
