package main

import (
	"encoding/json"
	"fmt"
	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
)

const Version = "0.1.0"

var jestDir, isInitialised, initErr = InitStatus()
var JestDir = jestDir
var IsInitialised = isInitialised
var JestDB, dbErr = OpenDB()
var Conf Config

var r *rand.Rand

func init() {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)

	hostname, _ := os.Hostname()
	fmt.Println("\nJest version", Version, "- http://"+hostname+":80")
	fmt.Println("Get enterprise support at: https://www.AltSrc.com/jest\n")
}

func main() {
	if dbErr != nil {
		log.Warn(dbErr)
	}

	if initErr != nil {
		log.Warn(initErr)
	}

	if IsInitialised == true {
		InitDB()
		Conf, _ = LoadConfig()
	}

	r := mux.NewRouter()

	r.HandleFunc("/init", GetInitEndpoint).Methods("GET")
	r.HandleFunc("/init", CreateInitEndpoint).Methods("POST")
	r.HandleFunc("/init", DeleteInitEndpoint).Methods("DELETE")

	r.HandleFunc("/templates", ListTemplatesEndpoint).Methods("GET")
	r.HandleFunc("/templates", DeleteInitEndpoint).Methods("POST")
	r.HandleFunc("/templates/{name}", GetTemplateEndpoint).Methods("GET")
	r.HandleFunc("/templates/{name}", DeleteInitEndpoint).Methods("POST")
	r.HandleFunc("/templates/{name}", DeleteInitEndpoint).Methods("PUT")
	r.HandleFunc("/templates/{name}", DeleteInitEndpoint).Methods("DELETE")

	r.HandleFunc("/jails", ListJailsEndpoint).Methods("GET")
	r.HandleFunc("/jails", CreateJailsEndpoint).Methods("POST")
	r.HandleFunc("/jails", ChangeJailStateEndpoint).Methods("PUT")
	r.HandleFunc("/jails/{name}", GetJailEndpoint).Methods("GET")
	r.HandleFunc("/jails/{name}", CreateJailsEndpoint).Methods("POST")
	r.HandleFunc("/jails/{name}", DeleteJailEndpoint).Methods("DELETE")

	r.HandleFunc("/snapshots", DeleteInitEndpoint).Methods("GET")
	r.HandleFunc("/snapshots", DeleteInitEndpoint).Methods("POST")
	r.HandleFunc("/snapshots/{name}", DeleteInitEndpoint).Methods("GET")
	r.HandleFunc("/snapshots/{name}", DeleteInitEndpoint).Methods("POST")
	r.HandleFunc("/snapshots/{name}", DeleteInitEndpoint).Methods("PUT")
	r.HandleFunc("/snapshots/{name}", DeleteInitEndpoint).Methods("DELETE")

	r.HandleFunc("/config", DeleteInitEndpoint).Methods("GET")
	r.HandleFunc("/config", DeleteInitEndpoint).Methods("POST")
	r.HandleFunc("/config", DeleteInitEndpoint).Methods("PUT")

	http.Handle("/", r)

	log.Fatal(http.ListenAndServe(":80", r))

	JestDB.Close()
}

// Create buckets in the database if they don't exist
func InitDB() {
	buckets := []string{"jails", "templates", "config"}

	for i := range buckets {
		err := JestDB.Update(func(tx *bolt.Tx) error {
			_, err := tx.CreateBucketIfNotExists([]byte(buckets[i]))
			if err != nil {
				return fmt.Errorf("create bucket: %s", err)
			}
			return nil
		})
		if err != nil {
			panic(err)
		}
	}
}

func InitStatus() (string, bool, error) {
	path, err := SearchZFSProperties("jest:dir")
	if err != nil {
		return "Not set", false, err
	}

	return path, true, nil
}

func OpenDB() (*bolt.DB, error) {
	if IsInitialised == true {
		db, err := bolt.Open(filepath.Join(JestDir, "JestDB.bolt"), 0600, nil)
		if err != nil {
			log.Fatal(err)
			return db, err
		}
		return db, nil
	}

	return &bolt.DB{}, fmt.Errorf("Host not initialised - cannot load JestDB")
}

func HostNotInitialised(w http.ResponseWriter, r *http.Request) {
	if IsInitialised == false {
		json.NewEncoder(w).Encode(fmt.Errorf("You must initialise the host before you can call this function."))
		return
	}
}
