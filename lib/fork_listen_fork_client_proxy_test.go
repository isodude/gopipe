package lib

import (
	"fmt"
	"net"
	"testing"
)

func TestFDPassthrough(t *testing.T) {
	payload := "hello there"
	pipe := &Pipe{}
	_, err := pipe.Unixpair()
	if err != nil {
		t.Fatalf("%v", err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("%v", err)
	}
	ch := make(chan error)
	go func(ch chan error) {
		conn, err := ln.Accept()
		if err != nil {
			ch <- err
			return
		}
		file, err := conn.(*net.TCPConn).File()
		if err != nil {
			ch <- err
			return
		}
		if err = PutFd(pipe.Fds[0], file); err != nil {
			ch <- err
			return
		}
		file, err = GetFd(pipe.Fds[1], "socket")
		if err != nil {
			ch <- err
			return
		}
		conn, err = net.FileConn(file)
		if err != nil {
			ch <- err
			return
		}
		buf := make([]byte, len(payload))
		n, err := conn.Read(buf)
		if err != nil {
			ch <- err
			return
		}
		if n != len(buf) {
			ch <- fmt.Errorf("n(%d) != len(buf)(%d)", n, len(buf))
			return

		}
		if string(payload) != string(buf) {
			ch <- fmt.Errorf("string(payload)(%s) != string(buf)(%s)", string(payload), string(buf))
			return
		}
		conn.Close()
		ln.Close()
		ch <- nil
		close(ch)
	}(ch)
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("%v", err)
	}
	n, err := conn.Write([]byte(payload))
	if err != nil {
		t.Fatalf("%v", err)
	}

	if n != len(payload) {
		t.Fatalf("n(%d) != len(buf)(%d)", n, len(payload))
	}
	if err = <-ch; err != nil {
		t.Fatalf("%v", err)
	}
}

func TestFDPassthrough2(t *testing.T) {
	payload := "hello there"
	pipe1 := &Pipe{}
	c1, err := pipe1.Unixpair()
	if err != nil {
		t.Fatalf("%v", err)
	}
	pipe2 := &Pipe{}
	c2, err := pipe2.Unixpair()
	if err != nil {
		t.Fatalf("%v", err)
	}
	ch := make(chan error)
	go func(ch chan error) {
		for {
			p := make([]byte, 0)
			oob := make([]byte, 24)
			_, _, _, _, err := c1[1].(*net.UnixConn).ReadMsgUnix(p, oob)
			if err != nil {
				ch <- err
				return
			}
			fmt.Printf("debug: %v, %v\n", oob, err)
			_, _, err = c2[0].(*net.UnixConn).WriteMsgUnix(p, oob, nil)
			if err != nil {
				ch <- err
				return
			}
		}
	}(ch)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("%v", err)
	}

	go func(ch chan error) {
		conn, err := ln.Accept()
		if err != nil {
			ch <- err
			return
		}
		defer conn.Close()
		file, err := conn.(*net.TCPConn).File()
		if err != nil {
			ch <- err
			return
		}
		/*
			fd, err := ConnFd(c1[0].(syscall.Conn))
			if err != nil {
				ch <- err
				return
			}
		*/
		if err = PutFd(int(pipe1.Files[0].Fd()), file); err != nil {
			ch <- err
			return
		}
		/*
			fd, err = ConnFd(c2[1].(syscall.Conn))
			if err != nil {
				ch <- err
				return
			}
		*/
		file, err = Get(c2[1].(*net.UnixConn), "socket")
		if err != nil {
			ch <- err
			return
		}
		conn, err = net.FileConn(file)
		if err != nil {
			ch <- err
			return
		}
		defer conn.Close()
		buf := make([]byte, len(payload))
		n, err := conn.Read(buf)
		if err != nil {
			ch <- err
			return
		}
		if n != len(buf) {
			ch <- fmt.Errorf("n(%d) != len(buf)(%d)", n, len(buf))
			return
		}
		if string(payload) != string(buf) {
			ch <- fmt.Errorf("string(payload)(%s) != string(buf)(%s)", string(payload), string(buf))
			return
		}
		ln.Close()
		ch <- nil
		close(ch)
	}(ch)
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("%v", err)
	}
	n, err := conn.Write([]byte(payload))
	if err != nil {
		t.Fatalf("%v", err)
	}

	if n != len(payload) {
		t.Fatalf("n(%d) != len(buf)(%d)", n, len(payload))
	}
	if err = <-ch; err != nil {
		t.Fatalf("%v", err)
	}
}
