package main

import (
	"net/http"
)

func main() {
	http.Handle("/", http.FileServer(http.Dir("./")))
	http.ListenAndServe(":8080", nil)
}

// package main

// import (
//   "log"
//   "net/http"
// )

// func main() {
//   log.Fatal(http.ListenAndServe(":8080", http.FileServer(http.Dir("."))))
// }