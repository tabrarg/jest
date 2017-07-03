package main

import (
	"net/http"
	"fmt"
)

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Jest Version:", Version, "Documentation:")
}
