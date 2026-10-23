package jnet_test

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/JohnEsleyer/barista/pkg/runtime/java/lang"
	jnet "github.com/JohnEsleyer/barista/pkg/runtime/java/net"
)

func TestHttpServerAndClient(t *testing.T) {
	server := jnet.HttpServerCreate(jnet.NewInetSocketAddress(0), 0)
	server.CreateContext(lang.NewString("/api/greet"), func(exchange *jnet.HttpExchange) {
		method := exchange.GetRequestMethod()
		response := fmt.Sprintf(`{"status":"ok","method":"%s"}`, method.GoString())
		bytes := lang.NewString(response).GetBytes()
		exchange.GetResponseHeaders().Set(lang.NewString("Content-Type"), lang.NewString("application/json"))
		exchange.SendResponseHeaders(200, int64(len(bytes)))
		out := exchange.GetResponseBody()
		out.Write(bytes)
		out.Close()
	})
	server.Start()
	defer server.Stop(0)

	port := server.GetAddress().GetPort()
	if port <= 0 {
		t.Fatalf("expected bound port > 0, got %d", port)
	}

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/greet", port))
	if err != nil {
		t.Fatalf("http GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	want := `{"status":"ok","method":"GET"}`
	if string(body) != want {
		t.Errorf("got body %q, want %q", string(body), want)
	}
}

func TestHttpServerEchoPost(t *testing.T) {
	server := jnet.HttpServerCreate(jnet.NewInetSocketAddress(0), 0)
	server.CreateContext(lang.NewString("/api/echo"), func(exchange *jnet.HttpExchange) {
		in := exchange.GetRequestBody()
		body := in.ReadAllBytes()
		exchange.SendResponseHeaders(200, int64(len(body)))
		out := exchange.GetResponseBody()
		out.Write(body)
		out.Close()
	})
	server.Start()
	defer server.Stop(0)

	port := server.GetAddress().GetPort()
	resp, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/api/echo", port), "text/plain", strings.NewReader("Barista Native REST"))
	if err != nil {
		t.Fatalf("http POST: %v", err)
	}
	defer resp.Body.Close()

	resBody, _ := io.ReadAll(resp.Body)
	if string(resBody) != "Barista Native REST" {
		t.Errorf("echo body = %q, want \"Barista Native REST\"", string(resBody))
	}
}

func TestSocketAndServerSocket(t *testing.T) {
	serverSocket := new(jnet.ServerSocket)
	serverSocket.Init(int32(0))
	defer serverSocket.Close()

	port := serverSocket.GetLocalPort()
	if port <= 0 {
		t.Fatalf("server socket local port = %d", port)
	}

	done := make(chan bool)
	go func() {
		client := new(jnet.Socket)
		client.Init(lang.NewString("127.0.0.1"), port)
		defer client.Close()

		msg := lang.NewString("PING").GetBytes()
		client.GetOutputStream().Write(msg)

		buf := make([]int8, 4)
		client.GetInputStream().Read(buf)
		got := lang.NewString("").Init(buf).GoString()
		if got != "PONG" {
			t.Errorf("client received %q, want PONG", got)
		}
		done <- true
	}()

	conn := serverSocket.Accept()
	defer conn.Close()

	buf := make([]int8, 4)
	conn.GetInputStream().Read(buf)
	received := lang.NewString("").Init(buf).GoString()
	if received != "PING" {
		t.Fatalf("server received %q, want PING", received)
	}

	reply := lang.NewString("PONG").GetBytes()
	conn.GetOutputStream().Write(reply)
	<-done
}

func TestByteArrayStreams(t *testing.T) {
	out := new(jnet.ByteArrayOutputStream)
	out.Init()
	out.Write(lang.NewString("Barista Byte Array").GetBytes())
	if out.Size() != 18 {
		t.Errorf("Size() = %d, want 18", out.Size())
	}
	data := out.ToByteArray()

	in := new(jnet.ByteArrayInputStream)
	in.Init(data)
	readAll := in.ReadAllBytes()
	got := lang.NewString("").Init(readAll).GoString()
	if got != "Barista Byte Array" {
		t.Errorf("stream read = %q, want \"Barista Byte Array\"", got)
	}
}

func TestURI(t *testing.T) {
	uri := jnet.URICreate(lang.NewString("http://127.0.0.1:8080/api/users?page=1"))
	if uri.GetPath().GoString() != "/api/users" {
		t.Errorf("GetPath() = %q, want /api/users", uri.GetPath().GoString())
	}
	if uri.GetQuery().GoString() != "page=1" {
		t.Errorf("GetQuery() = %q, want page=1", uri.GetQuery().GoString())
	}
	if uri.GetPort() != 8080 {
		t.Errorf("GetPort() = %d, want 8080", uri.GetPort())
	}
}
