package main

import (
	"net/http"
	"fmt"
	"github.com/gorilla/mux"
)

func JailHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	fmt.Println("OK")
}

func JIDHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	w.WriteHeader(http.StatusOK)
	fmt.Println(vars["jid"], ": OK")
}
