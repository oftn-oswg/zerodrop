package main

import (
	"net/http"
)

type NotFoundHandler struct{}

func (n NotFoundHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "File not found", 200)
}
