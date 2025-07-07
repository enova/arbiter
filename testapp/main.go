package main

import (
	"log"
	"net/http"
	"os"

	"github.com/enova/arbiter"
)

func main() {
	backends := arbiter.NewBackendList()
	backends.AddState("be_1", os.DirFS("./be_1"))
	backends.AddState("be_2", os.DirFS("./be_2"))

	h, err := arbiter.NewHandler(backends, log.Default())
	if err != nil {
		panic(err)
	}

	log.Fatal(http.ListenAndServe("localhost:6060", h))
}
