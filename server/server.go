package server

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path"
	"server/server/cgzip"
	"strconv"
	"strings"
)

var i int = 0

type Body interface {
	Read(p []byte) (n int, err error)
	Write(p []byte) (n int, err error)
}

type Response struct {
	// body of the response
	body Body

	// status of the response
	status string

	// protocol used
	protocol string

	// headers of the response
	headers map[string]string

	// static file flag
	static bool
}

type Request struct {
	method string

	protocol string

	path string

	headers map[string]string
}

type Handler struct {
	Routes map[string]func(Request, *Response)
}

func resize(size int) []byte {
	return make([]byte, size)
}

func page404(conn net.Conn, req Request, res *Response) {
	res.SetStatus(404, "PAGE NOT FOUND")
	res.WriteBody([]byte("<html><body><h1>PAGE NOT FOUND</h1></body></html>"))
}

func parseRequest(conn net.Conn) (Request, Response, error) {
	req, res := Request{headers: make(map[string]string)}, Response{headers: make(map[string]string), body: &bytes.Buffer{}}

	// request parsing
	r := bufio.NewReader(conn)

	// first line contains method path and protocol
	line, err := r.ReadString('\n')
	line = strings.Trim(line, "\r\n")

	if err == io.EOF && len(line) == 0 {
		return req, res, io.EOF
	}

	re := strings.Split(string(line), " ")

	req.method, req.path, req.protocol = re[0], re[1], re[2]

	// parsing headers
	for {
		line, _ := r.ReadString('\n')
		if string(line) == "\r\n" {
			break
		}
		proto := strings.Split(string(line), ": ")
		req.headers[proto[0]] = strings.Trim(proto[1], "\r\n")
	}

	// for k, v := range req.headers {
	// 	fmt.Println("header ", k, " ", v)
	// }

	// response creation
	res.protocol = "HTTP/1.1"
	return req, res, nil
}

func CreateServer() *Handler {
	return &Handler{Routes: make(map[string]func(Request, *Response))}
}

func (r *Response) SetStatus(statusCode int, message string) {
	r.status = fmt.Sprintf("%d %s", statusCode, message)
}

func (r *Response) WriteHeader(key string, value string) {
	r.headers[key] = value
}
func (r *Response) WriteBody(value []byte) {

	r.body.Write(value)
	// r.body = append(r.body, fmt.Sprintf("%s", string(value))...)
	// if len(value) > cap(r.body)-len(r.body) {
	// 	newBuf := resize(cap(r.body) + (cap(r.body) - len(value)) + 512)
	// 	copy(newBuf, r.body)
	// 	r.body = newBuf
	// }
	// log.Println(len(r.body), len(value), string(r.body))
	// copy(r.body[len(r.body)+1:], value)
}

func (r *Response) compress(algo string) {

	buff := &bytes.Buffer{}
	// wr := gzip.NewWriter(buff)
	wr := cgzip.NewWriter(buff)

	f, err := ioutil.ReadAll(r.body)

	if err != nil {
		fmt.Println(err)
	}

	wr.Write(f)

	wr.Close()

	r.WriteHeader("Content-Length", fmt.Sprintf("%v", buff.Len()))

	r.body = buff

}

func (r *Response) generateHeaders(req Request) (response []byte) {
	response = make([]byte, 0)

	// r.WriteHeader("server", "hero-0.0.1")
	// r.WriteHeader("Connection", "keep-alive")

	// if the file is from static folder
	if r.static == true {

		// open the static file given in path ex: /static/index.html so we pass path from index 1 to avoud '/'
		f, err := os.Open(req.path[1:])

		defer (func() {
			if r := recover(); r != nil {
				f.Close()
			}
		})()

		if err != nil {
			panic(err)
		}

		// stat is used to get the size of the file for Content-Length Header
		stat, err := f.Stat()
		r.WriteHeader("Content-Length", fmt.Sprintf("%v", stat.Size()))
		r.WriteHeader("Accept-Ranges", "bytes")

		var loff, _ string = "0", "0"

		// partial requests i.e, http request code 206 has a range header which we need to handle
		// it is of the format
		// Range: bytes={left_offset}-{right_offset}
		if req.headers["Range"] != "" {
			ranges := strings.Split(req.headers["Range"], "=")[1]
			off := strings.Split(ranges, "-")
			switch len(off) {
			case 0:
				panic("invalid format of range")
			case 1:
				loff = off[0]
			case 2:
				loff, _ = off[0], off[1]
			}

			i, _ := strconv.ParseInt(loff, 10, 64)
			f.Seek(i, 0)

		}

		// content type
		switch path.Ext(f.Name()) {
		case ".html":
			r.WriteHeader("Content-Type", "text/html")
			r.WriteHeader("Content-Encoding", "gzip")
		case ".mp4":
			r.SetStatus(206, "Partial Content")
			r.WriteHeader("Content-Type", "video/mp4")
			iloff, _ := strconv.ParseInt(loff, 10, 64)
			r.WriteHeader("Content-Length", fmt.Sprintf("%v", stat.Size()-iloff))
			r.WriteHeader("Content-Range", fmt.Sprintf("bytes %s-%v/%v", loff, stat.Size()-1, stat.Size()))
		case ".js":
			r.WriteHeader("Content-Type", "application/javascript")
		}

		// donot compress
		r.body = f

		if r.headers["Content-Encoding"] != "" {
			r.compress("gzip")
		}

	}

	// appends the protocol and status to the response
	response = append(response, fmt.Sprintf("%s %s\r\n", r.protocol, r.status)...)
	// fmt.Println("clear", string(response))

	// appending headers to the response
	for key, val := range r.headers {
		response = append(response, []byte(fmt.Sprintf("%s: %s\r\n", key, val))...)
	}

	// ending line for headers
	response = append(response, []byte("\r\n")...)

	return
}

func (r *Response) generateBody(conn net.Conn, req Request) {
	buf := make([]byte, 1024*64)

	for {
		n, err := r.body.Read(buf)
		if n != 0 {
			_, err := conn.Write(buf[:n])
			if err != nil {
				return
			}
		}
		if err == io.EOF {
			break
		}
	}

	return
}

func (h *Handler) HandleFunc(route string, handler func(Request, *Response)) {
	h.Routes[route] = handler
}

func (h *Handler) handleRequests(conn net.Conn) {

	// parse incoming request and generating the req and response objects
	req, res, err := parseRequest(conn)

	defer exceptionHandler(conn, req, &res, h)

	if err != nil {
		panic("Malformed Reqest")
		return
	}

	// routes handler
	if h.Routes[req.path] != nil {
		h.Routes[req.path](req, &res)
	} else if strings.Split(req.path[1:], "/")[0] == "static" {
		// log.Println(strings.Split(req.path[1:], "/")[1:])
		res.static = true
	} else {
		res.WriteHeader("Content-Type", "text/html")
		panic("Resource Not Found")
	}
	// log.Println(req.path)

	if res.status == "" {
		res.SetStatus(200, "OK")
	}
	requestEnd(req, &res, conn)
	// conn.Write(res.generateHeaders(req))
	// res.generateBody(conn, req, i)
}

func requestEnd(req Request, res *Response, conn net.Conn) {
	conn.Write(res.generateHeaders(req))
	res.generateBody(conn, req)
	conn.Close()
}

func exceptionHandler(conn net.Conn, req Request, res *Response, h *Handler) {
	if r := recover(); r != nil {
		fmt.Println("Recovered from panic", r)
		switch r {
		case "Resource Not Found":
			page404(conn, req, res)
		case "Malformed Request":
			res.SetStatus(400, "Bad Request")
		}
	}
	requestEnd(req, res, conn)
}

func (h *Handler) Listen(addr string, port int) {
	// fmt.Println("Attempting to start server")
	l, err := net.Listen("tcp", fmt.Sprintf("%s:%s", addr, strconv.Itoa(port)))
	if err != nil {
		panic(err)
	}
	fmt.Println("Started server at location", addr, " : ", port)
	for {
		if err != nil {
			panic(err)
		}
		conn, err := l.Accept()
		// log.Println("new connection")
		if err != nil {
			panic(err)
		}
		go h.handleRequests(conn)
	}
}
