package lib

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"
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
		if err := CopyUnix(c2[0], c1[1]); err != nil {
			ch <- err
			return
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
		if err = Put(c1[0].(*net.UnixConn), file); err != nil {
			ch <- err
			return
		}
		// It's actually needed to use the FileConn here and not Fds or Files from pipe2
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
func TestFDPassthrough3(t *testing.T) {
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
		if err := CopyUnix(c2[0], c1[1]); err != nil {
			ch <- err
			return
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
		if err = PutFd(int(pipe1.Files[0].Fd()), file); err != nil {
			ch <- err
			return
		}
		// It's actually needed to use the FileConn here and not Fds or Files from pipe2
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

func TestForkListenForkClientE2E(t *testing.T) {
	ch := make(chan struct{})
	connCh := make(chan net.Conn)

	ln1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("%v", err)
	}
	addr1 := ln1.Addr().String()
	ln1.Close()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("%v", err)
	}
	addr := fmt.Sprintf("--client.addr=%s", ln.Addr().String())

	defer ln.Close()
	ctx, cancel := context.WithCancelCause(context.Background())
	go func(connCh chan net.Conn) {
		defer ln.Close()
		defer close(connCh)
		close(ch)
		conn, err := ln.Accept()
		if err != nil {
			cancel(err)
			return
		}
		connCh <- conn
		cancel(nil)
	}(connCh)

	payload := "hello there"

	<-ch

	if len(ctx.Done()) > 0 {
		t.Fatalf("%v", ctx.Err())
	}

	args := []string{"-test.run", "^TestE2EBin$", "-test.timeout", "20s", "--", "--listen.fork", fmt.Sprintf("--listen.addr=%s", addr1), "--client.fork", addr}
	l := exec.CommandContext(ctx, os.Args[0], args...)
	l.Env = []string{
		`CMD_TEST_E2E=1`,
		`CMD_LISTEN_FORK_ARGS=-test.run ^TestE2EBin$ -test.timeout 20s --`,
		`CMD_CLIENT_FORK_ARGS=-test.run ^TestE2EBin$ -test.timeout 20s --`,
	}
	l.Stdout, l.Stderr = os.Stdout, os.Stderr

	if err := l.Start(); err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			t.Fatalf("unable to start process: %s, %v, %v, %s", os.Args[0], args, err, err.Stderr)
		}
		t.Fatalf("%v", err)
	}

	if len(ctx.Done()) > 0 {
		t.Fatalf("%v", ctx.Err())
	}

	retries := 0
retry:
	connWrite, err := net.Dial("tcp", addr1)
	if err != nil {
		if retries > 10 {
			t.Fatalf("unable to dial: %v", err)
		} else {
			retries++
			time.Sleep(100 * time.Millisecond)
			goto retry
		}
	}
	defer connWrite.Close()

	_, err = connWrite.Write([]byte(payload))
	if err != nil {
		t.Fatalf("%v", err)
	}

	select {
	case <-ctx.Done():
		t.Fatalf("%v", ctx.Err())
	case conn := <-connCh:
		defer conn.Close()
		buf := make([]byte, len(payload))
		_, err := conn.Read(buf)
		if err != nil {
			t.Fatalf("%v", err)
		}
		if string(buf) != payload {
			t.Fatalf("payload not received")
		}
		l.Cancel()
	}
	fmt.Printf("hey: %v\n", l.Wait())
}
