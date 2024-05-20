package lib

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
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

// TestE2EBin isn't a real test.
func TestE2EBinListen(t *testing.T) {
	if os.Getenv("CMD_TEST_E2E") != "1" {
		t.SkipNow()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	p := &UnixSendProxy{}
	l := &Listen{
		Ctx:      context.Background(),
		Protocol: "tcp",
		Addr: &Addr{
			Addr: "127.0.0.1:9000",
		},
		TLS: ListenTLS{
			ClientTLS: &ClientTLS{},
		},
	}
	c := &Client{
		Ctx: context.Background(),
		Addr: &Addr{
			Addr: "FD:3",
		},
	}
	go func() {
		defer stop()
		if err := p.Proxy(l, c); err != nil {
			fmt.Printf("err: %v\n", err)
		}
	}()

	<-ctx.Done()
}

func TestE2EBinClient(t *testing.T) {
	if os.Getenv("CMD_TEST_E2E") != "1" {
		t.SkipNow()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	p := &UnixDialProxy{}
	l := &Listen{
		Ctx:      ctx,
		Protocol: "tcp",
		Addr: &Addr{
			Addr: "FD:3",
		},
		IncomingConn: true,
	}
	c := &Client{
		Ctx:      ctx,
		Protocol: "tcp",
		SourceIP: "127.0.0.1",
		Addr: &Addr{
			Addr: "127.0.0.1:9001",
		},
		NetNs: NetworkNamespace{
			Ctx:     context.Background(),
			Disable: true,
		},
		Timeout: 2 * time.Second,
	}
	go func() {
		defer stop()
		if err := p.Proxy(l, c); err != nil {
			fmt.Printf("err: %v\n", err)
		}
	}()

	<-ctx.Done()
}

// TestE2EBin isn't a real test.
func TestE2EBin(t *testing.T) {
	if os.Getenv("CMD_TEST_E2E") != "1" {
		t.SkipNow()
	}

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(3)
	}

	MainFunc(args)
}

func TestE2E(t *testing.T) {
	ch := make(chan error, 2)
	ch1 := make(chan struct{})
	connCh := make(chan net.Conn)
	ln, err := net.Listen("tcp", "127.0.0.1:9001")
	if err != nil {
		t.Fatalf("%v", err)
	}
	go func(ch chan error, connCh chan net.Conn) {
		defer ln.Close()
		defer close(ch)
		defer close(connCh)
		close(ch1)
		conn, err := ln.Accept()
		if err != nil {
			ch <- err
			return
		}
		connCh <- conn
	}(ch, connCh)

	payload := "hello there"

	<-ch1

	p1, p2 := &Pipe{}, &Pipe{}
	c1, err := p1.Unixpair()
	if err != nil {
		t.Fatalf("%v", err)
	}
	c2, err := p2.Unixpair()
	if err != nil {
		t.Fatalf("%v", err)
	}
	go CopyUnix(c2[0].(*net.UnixConn), c1[1].(*net.UnixConn))

	ctx := context.Background()
	args := []string{"-test.run", "^TestE2EBinListen$", "-test.timeout", "5s", "--"}
	l := exec.CommandContext(ctx, os.Args[0], args...)
	l.Env = []string{
		`CMD_TEST_E2E=1`,
	}
	f, _ := c1[0].(*net.UnixConn).File()
	l.ExtraFiles = []*os.File{f}

	l.Stdout, l.Stderr = os.Stdout, os.Stderr
	if err := l.Start(); err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			t.Fatalf("unable to start process: %s, %v, %v, %s", os.Args[0], args, err, err.Stderr)
		}
		t.Fatalf("%v", err)
	}
	defer l.Process.Signal(os.Interrupt)

	args = []string{"-test.run", "^TestE2EBinClient$", "-test.timeout", "5s", "--"}
	c := exec.CommandContext(ctx, os.Args[0], args...)
	c.Env = []string{
		`CMD_TEST_E2E=1`,
	}
	f, _ = c2[1].(*net.UnixConn).File()
	c.ExtraFiles = []*os.File{f}

	c.Stdout, c.Stderr = os.Stdout, os.Stderr
	if err := c.Start(); err != nil {
		if err, ok := err.(*exec.ExitError); ok {
			t.Fatalf("unable to start process: %s, %v, %v, %s", os.Args[0], args, err, err.Stderr)
		}
		t.Fatalf("%v", err)
	}
	defer l.Process.Signal(os.Interrupt)

	if len(ch) > 0 {
		t.Fatalf("%v", <-ch)
	}

	retries := 0
retry:
	connWrite, err := net.Dial("tcp", "127.0.0.1:9000")
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
	case err := <-ch:
		t.Fatalf("%v", err)
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
	}
}
