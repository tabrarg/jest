package main

import (
	"errors"
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

var JestDir string
var Initialised bool
var DB *bolt.DB

type jail struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

var r *rand.Rand

func init() {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

func main() {
	hostname, _ := os.Hostname()
	fmt.Println("\nJest version", Version, "- http://"+hostname+":8080")
	fmt.Println("Get enterprise support at: https://www.AltSrc.com/jest\n")

	OpenDB()

	r := mux.NewRouter()

	r.HandleFunc("/init", GetInitEndpoint).Methods("GET")
	r.HandleFunc("/init", CreateInitEndpoint).Methods("POST")
	r.HandleFunc("/init", DeleteInitEndpoint).Methods("DELETE")

	r.HandleFunc("/templates", DeleteInitEndpoint).Methods("GET")
	r.HandleFunc("/templates", DeleteInitEndpoint).Methods("POST")
	r.HandleFunc("/templates/{name}", DeleteInitEndpoint).Methods("GET")
	r.HandleFunc("/templates/{name}", DeleteInitEndpoint).Methods("POST")
	r.HandleFunc("/templates/{name}", DeleteInitEndpoint).Methods("PUT")
	r.HandleFunc("/templates/{name}", DeleteInitEndpoint).Methods("DELETE")

	r.HandleFunc("/jails", DeleteInitEndpoint).Methods("GET")
	r.HandleFunc("/jails", CreateJailsEndpoint).Methods("POST")
	r.HandleFunc("/jails/{name}", DeleteInitEndpoint).Methods("GET")
	r.HandleFunc("/jails/{name}", DeleteInitEndpoint).Methods("POST")
	r.HandleFunc("/jails/{name}", DeleteInitEndpoint).Methods("PUT")
	r.HandleFunc("/jails/{name}", DeleteInitEndpoint).Methods("DELETE")

	r.HandleFunc("/snapshots", DeleteInitEndpoint).Methods("GET")
	r.HandleFunc("/snapshots", DeleteInitEndpoint).Methods("POST")
	r.HandleFunc("/snapshots/{name}", DeleteInitEndpoint).Methods("GET")
	r.HandleFunc("/snapshots/{name}", DeleteInitEndpoint).Methods("POST")
	r.HandleFunc("/snapshots/{name}", DeleteInitEndpoint).Methods("PUT")
	r.HandleFunc("/snapshots/{name}", DeleteInitEndpoint).Methods("DELETE")

	http.Handle("/", r)

	log.Fatal(http.ListenAndServe(":8080", r))
}

func InitStatus() (string, bool, error) {
	path, err := SearchZFSProperties("jest:dir")
	if err != nil {
		return "Not set", false, err
	}

	return path, true, nil
}

func OpenDB() error {
	JestDir, Initialised, err := InitStatus()
	if err != nil {
		log.Warn(err)
	}
	log.Info("Path: ", JestDir, ", Init Status: ", Initialised)

	if Initialised == true {
		DB, err := bolt.Open(filepath.Join(JestDir, "JestDB.bolt"), 0600, nil)
		if err != nil {
			log.Fatal(err)
			return err
		}
		defer DB.Close()
		return nil
	}

	return errors.New("Host not initialised - cannot load DB")
}
