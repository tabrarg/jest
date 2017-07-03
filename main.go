package main

import (
	"github.com/gorilla/mux"
	"net/http"
	"fmt"
	"os"
	"time"
	"math/rand"
	log "github.com/sirupsen/logrus"
)

const Version = "0.1.0"

type jail struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
}

var r *rand.Rand

func init() {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	// Log as JSON instead of the default ASCII formatter.
	//log.SetFormatter(&log.JSONFormatter{})

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	log.SetOutput(os.Stdout)

	// Only log the warning severity or above.
	log.SetLevel(log.DebugLevel)
}

func main() {
	r := mux.NewRouter()

	r.HandleFunc("/", HomeHandler)
	r.HandleFunc("/jail", JailHandler)
	r.HandleFunc("/jail/{jid}", JIDHandler)

	r.HandleFunc("/initialise", GetInitEndpoint).Methods("GET")
	r.HandleFunc("/initialise", CreateInitEndpoint).Methods("POST")
	r.HandleFunc("/initialise", DeleteInitEndpoint).Methods("DELETE")

	http.Handle("/", r)

	hostname, _ := os.Hostname()
	fmt.Println("Jest version", Version, "- http://" + hostname + ":8080")
	fmt.Println("Get enterprise support at: https://www.AltSrc.com/jest")
	log.Fatal(http.ListenAndServe(":8080", r))
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