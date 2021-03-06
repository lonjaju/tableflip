package tableflip

import (
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFdsListen(t *testing.T) {
	socketPath, cleanup := tempSocket(t)
	defer cleanup()

	addrs := [][2]string{
		{"unix", socketPath},
		{"tcp", "localhost:0"},
	}

	fds := newFds(nil)

	for _, addr := range addrs {
		ln, err := fds.Listen(addr[0], addr[1])
		if err != nil {
			t.Fatal(err)
		}
		if ln == nil {
			t.Fatal("Missing listener", addr)
		}
		ln.Close()
	}
}

func tempSocket(t *testing.T) (string, func()) {
	t.Helper()

	temp, err := ioutil.TempDir("", "tableflip")
	if err != nil {
		t.Fatal(err)
	}

	return filepath.Join(temp, "socket"), func() { os.RemoveAll(temp) }
}

func TestFdsListener(t *testing.T) {
	addr := &net.TCPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 0,
	}

	tcp, err := net.ListenTCP("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer tcp.Close()

	socketPath, cleanup := tempSocket(t)
	defer cleanup()
	unix, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close()

	parent := newFds(nil)
	if err := parent.AddListener(addr.Network(), addr.String(), tcp); err != nil {
		t.Fatal("Can't add listener:", err)
	}
	tcp.Close()

	if err := parent.AddListener("unix", socketPath, unix.(Listener)); err != nil {
		t.Fatal("Can't add listener:", err)
	}
	unix.Close()

	if _, err := os.Stat(socketPath); err != nil {
		t.Error("Unix.Close() unlinked socketPath:", err)
	}

	// Linux supports the abstract namespace for domain sockets.
	if runtime.GOOS == "linux" {
		abstractUnix, err := parent.Listen("unix", "@tableflip-test-r5N5j")
		if err != nil {
			t.Fatal(err)
		}
		defer abstractUnix.Close()
	}

	child := newFds(parent.copy())
	ln, err := child.Listener(addr.Network(), addr.String())
	if err != nil {
		t.Fatal("Can't get listener:", err)
	}
	if ln == nil {
		t.Fatal("Missing listener")
	}
	ln.Close()

	child.closeInherited()
	if _, err := os.Stat(socketPath); err == nil {
		t.Error("closeInherited() did not unlink socketPath")
	}
}

func TestFdsUnixListener(t *testing.T) {
	temp, err := ioutil.TempDir("", "tableflip")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(temp)

	fds := newFds(nil)

	socketPath := filepath.Join(temp, "socket")
	unix, err := fds.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	unix.Close()

	fds.closeAndRemoveUsed()
	if _, err := os.Stat(socketPath); err == nil {
		t.Error("Unix listeners are not removed")
	}
}

func TestFdsConn(t *testing.T) {
	socketPath, cleanup := tempSocket(t)
	defer cleanup()
	unix, err := net.ListenUnixgram("unixgram", &net.UnixAddr{
		Net:  "unixgram",
		Name: socketPath,
	})
	if err != nil {
		t.Fatal(err)
	}

	parent := newFds(nil)
	if err := parent.AddConn("unixgram", "", unix); err != nil {
		t.Fatal("Can't add conn:", err)
	}
	unix.Close()

	child := newFds(parent.copy())
	conn, err := child.Conn("unixgram", "")
	if err != nil {
		t.Fatal("Can't get conn:", err)
	}
	if conn == nil {
		t.Fatal("Missing conn")
	}
	conn.Close()
}

func TestFdsFile(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	parent := newFds(nil)
	if err := parent.AddFile("test", w); err != nil {
		t.Fatal("Can't add file:", err)
	}
	w.Close()

	child := newFds(parent.copy())
	file, err := child.File("test")
	if err != nil {
		t.Fatal("Can't get file:", err)
	}
	if file == nil {
		t.Fatal("Missing file")
	}
	file.Close()
}
