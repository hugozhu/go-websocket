// Copyright 2012 Gary Burd
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

// Command server is a test server for the Autobahn WebSockets Test Suite.
package main

import (
	"errors"
	"flag"
	"github.com/garyburd/go-websocket/websocket"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

// echoCopy echoes messages from the client using io.Copy.
func echoCopy(w http.ResponseWriter, r *http.Request, writerOnly bool) {
	conn, err := websocket.Upgrade(w, r.Header, "", 4096, 4096)
	if err != nil {
		log.Println("Upgrade:", err)
		http.Error(w, "Bad request", 400)
		return
	}
	defer conn.Close()
	for {
		op, r, err := conn.NextReader()
		if err != nil {
			if err != io.EOF {
				log.Println("NextReader:", err)
			}
			return
		}
		if op == websocket.OpPong {
			continue
		}
		if op == websocket.OpText {
			r = &validator{r: r}
		}
		w, err := conn.NextWriter(op)
		if err != nil {
			log.Println("NextWriter:", err)
			return
		}
		if op == websocket.OpText {
			r = &validator{r: r}
		}
		if writerOnly {
			_, err = io.Copy(struct{ io.Writer }{w}, r)
		} else {
			_, err = io.Copy(w, r)
		}
		if err != nil {
			if err == errInvalidUTF8 {
				conn.WriteControl(websocket.OpClose,
					websocket.FormatCloseMessage(websocket.CloseInvalidFramePayloadData, ""),
					time.Time{})
			}
			log.Println("Copy:", err)
			return
		}
		err = w.Close()
		if err != nil {
			log.Println("Close:", err)
			return
		}
	}
}

func echoCopyWriterOnly(w http.ResponseWriter, r *http.Request) {
	echoCopy(w, r, true)
}

func echoCopyFull(w http.ResponseWriter, r *http.Request) {
	echoCopy(w, r, false)
}

// echoReadAll echoes messages from the client by reading the entire message
// with ioutil.ReadAll.
func echoReadAll(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Upgrade(w, r.Header, "", 4096, 4096)
	if err != nil {
		log.Println("Upgrade:", err)
		http.Error(w, "Bad request", 400)
		return
	}
	defer conn.Close()
	for {
		op, r, err := conn.NextReader()
		if err != nil {
			if err != io.EOF {
				log.Println("NextReader:", err)
			}
			return
		}
		if op == websocket.OpPong {
			continue
		}
		if op == websocket.OpText {
			r = &validator{r: r}
		}
		b, err := ioutil.ReadAll(r)
		if err != nil {
			if err == errInvalidUTF8 {
				conn.WriteControl(websocket.OpClose,
					websocket.FormatCloseMessage(websocket.CloseInvalidFramePayloadData, ""),
					time.Time{})
			}
			log.Println("ReadAll:", err)
			return
		}
		w, err := conn.NextWriter(op)
		if err != nil {
			log.Println("NextWriter:", err)
			return
		}
		if _, err := w.Write(b); err != nil {
			log.Println("Writer:", err)
			return
		}
		if err := w.Close(); err != nil {
			log.Println("Close:", err)
			return
		}
	}
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found.", 404)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, "<html><body>Echo Server</body></html")
}

var addr = flag.String("addr", ":9000", "http service address")

func main() {
	flag.Parse()
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/c", echoCopyWriterOnly)
	http.HandleFunc("/f", echoCopyFull)
	http.HandleFunc("/r", echoReadAll)
	err := http.ListenAndServe(*addr, nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

type validator struct {
	state int
	x     rune
	r     io.Reader
}

var errInvalidUTF8 = errors.New("invalid utf8")

func (r *validator) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	state := r.state
	x := r.x
	for _, b := range p[:n] {
		state, x = decode(state, x, b)
		if state == utf8Reject {
			break
		}
	}
	r.state = state
	r.x = x
	if state == utf8Reject || (err == io.EOF && state != utf8Accept) {
		return n, errInvalidUTF8
	}
	return n, err
}

// UTF-8 decoder from http://bjoern.hoehrmann.de/utf-8/decoder/dfa/
//
// Copyright (c) 2008-2009 Bjoern Hoehrmann <bjoern@hoehrmann.de>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to
// deal in the Software without restriction, including without limitation the
// rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
// sell copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS
// IN THE SOFTWARE.
var utf8d = [...]byte{
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // 00..1f
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // 20..3f
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // 40..5f
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, // 60..7f
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, 9, // 80..9f
	7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, 7, // a0..bf
	8, 8, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, 2, // c0..df
	0xa, 0x3, 0x3, 0x3, 0x3, 0x3, 0x3, 0x3, 0x3, 0x3, 0x3, 0x3, 0x3, 0x4, 0x3, 0x3, // e0..ef
	0xb, 0x6, 0x6, 0x6, 0x5, 0x8, 0x8, 0x8, 0x8, 0x8, 0x8, 0x8, 0x8, 0x8, 0x8, 0x8, // f0..ff
	0x0, 0x1, 0x2, 0x3, 0x5, 0x8, 0x7, 0x1, 0x1, 0x1, 0x4, 0x6, 0x1, 0x1, 0x1, 0x1, // s0..s0
	1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 1, 0, 1, 0, 1, 1, 1, 1, 1, 1, // s1..s2
	1, 2, 1, 1, 1, 1, 1, 2, 1, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 2, 1, 1, 1, 1, 1, 1, 1, 1, // s3..s4
	1, 2, 1, 1, 1, 1, 1, 1, 1, 2, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 3, 1, 3, 1, 1, 1, 1, 1, 1, // s5..s6
	1, 3, 1, 1, 1, 1, 1, 3, 1, 3, 1, 1, 1, 1, 1, 1, 1, 3, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, // s7..s8
}

const (
	utf8Accept = 0
	utf8Reject = 1
)

func decode(state int, x rune, b byte) (int, rune) {
	t := utf8d[b]
	if state != utf8Accept {
		x = rune(b&0x3f) | (x << 6)
	} else {
		x = rune((0xff >> t) & b)
	}
	state = int(utf8d[256+state*16+int(t)])
	return state, x
}
