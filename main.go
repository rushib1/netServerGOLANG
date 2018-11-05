package main

import (
	"server/server"
)

func helloHandler(req server.Request, res *server.Response) {
	res.SetStatus(200, "OK")
	res.WriteHeader("Content-Type", "text/html")
	res.WriteBody([]byte("<html><body><h2>facebook is great</h2></body><script src=\"static/temp.js\" ></script></html>"))
}

func movedPermanently(req server.Request, res *server.Response) {
	res.SetStatus(301, "MOVED PERMANENTLY")
	res.WriteHeader("Location", "http://www.google.com")
}

func main() {
	server := server.CreateServer()
	server.HandleFunc("/hello", helloHandler)
	server.HandleFunc("/move", movedPermanently)
	server.Listen("127.0.0.1", 8080)
}

//
