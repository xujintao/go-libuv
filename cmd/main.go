package main

import (
	"log"

	"github.com/xujintao/go-libuv/net/http"
	"github.com/xujintao/go-libuv/poll"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	http.ListenAndServe(":8080")
	poll.Wait()
}
