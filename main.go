package main

import (
	"github.com/gorilla/mux"
	"net/http"
	"fmt"
	"log"
	"os"
)

const Version = "0.1.0"

type jail struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
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
