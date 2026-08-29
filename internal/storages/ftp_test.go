package storages

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

// fakeFTP is a minimal in-process FTP server speaking just enough of the
// protocol (control channel, EPSV passive data connections, LIST, RETR,
// STOR, DELE, RMD, MKD, RNFR/RNTO) to exercise the real jlaffaye/ftp client.
type fakeFTP struct {
	mu       sync.Mutex
	files    map[string][]byte
	dirs     map[string]bool
	listener net.Listener
	commands []string
}

func newFakeFTP(t *testing.T) *fakeFTP {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeFTP{files: map[string][]byte{}, dirs: map[string]bool{"/": true}, listener: listener}
	go f.serve()
	t.Cleanup(func() { _ = listener.Close() })
	return f
}

func (f *fakeFTP) port() int {
	return f.listener.Addr().(*net.TCPAddr).Port
}

func (f *fakeFTP) seedFile(path, content string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cleaned := strings.Trim(path, "/")
	f.files[cleaned] = []byte(content)
	// Only the parent directories of a file become directories.
	f.mkdirParents(parentOf("/" + cleaned))
}

func (f *fakeFTP) seedDir(path string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mkdirParents(path)
}

func (f *fakeFTP) mkdirParents(path string) {
	cleaned := strings.Trim(path, "/")
	parts := strings.Split(cleaned, "/")
	current := ""
	for _, part := range parts {
		if part == "" {
			continue
		}
		current += "/" + part
		f.dirs[current] = true
	}
}

func (f *fakeFTP) hasFile(path string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.files[strings.Trim(path, "/")]
	return ok
}

func (f *fakeFTP) hasDir(path string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dirs["/"+strings.Trim(path, "/")]
}

func (f *fakeFTP) fileCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.files)
}

func (f *fakeFTP) serve() {
	for {
		conn, err := f.listener.Accept()
		if err != nil {
			return
		}
		go f.handleConn(conn)
	}
}

func (f *fakeFTP) handleConn(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	reader := bufio.NewReader(conn)
	write := func(line string) { _, _ = conn.Write([]byte(line + "\r\n")) }
	write("220 fake ftp ready")
	var renameFrom string
	cwd := "/"
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		command := strings.TrimSpace(line)
		f.mu.Lock()
		f.commands = append(f.commands, command)
		f.mu.Unlock()
		upper := strings.ToUpper(command)
		fields := strings.Fields(command)
		verb := upper
		arg := ""
		if len(fields) > 1 {
			verb = strings.ToUpper(fields[0])
			arg = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(command), fields[0]))
		}
		switch verb {
		case "USER":
			write("331 password required")
		case "PASS":
			write("230 logged in")
		case "FEAT":
			write("211-Features:")
			write(" UTF8")
			write("211 End")
		case "SYST":
			write("215 UNIX Type: L8")
		case "TYPE":
			write("200 type set")
		case "OPTS":
			write("200 ok")
		case "NOOP":
			write("200 noop")
		case "PWD":
			write("257 \"/\" is cwd")
		case "CWD":
			target := resolve(cwd, arg)
			f.mu.Lock()
			_, ok := f.dirs[target]
			f.mu.Unlock()
			if !ok {
				write("550 no such directory")
				continue
			}
			cwd = target
			write("250 directory changed")
		case "EPSV":
			data, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				write("425 cannot open data connection")
				continue
			}
			pendingDataListeners.Store("current", data)
			port := data.Addr().(*net.TCPAddr).Port
			write(fmt.Sprintf("229 Entering Extended Passive Mode (|||%d|)", port))
		case "LIST":
			write("150 here comes the listing")
			data, err := acceptPending()
			if err != nil {
				write("425 data connection failed")
				continue
			}
			f.mu.Lock()
			dir := resolve(cwd, strings.TrimSpace(strings.TrimPrefix(arg, "-a")))
			var listing strings.Builder
			for path, content := range f.files {
				if parentOf("/"+path) == dir {
					fmt.Fprintf(&listing, "-rw-r--r-- 1 owner group %d Jan 02 10:00 %s\n", len(content), baseOf("/"+path))
				}
			}
			for path := range f.dirs {
				if path == "/" {
					continue
				}
				if parentOf(path) == dir {
					fmt.Fprintf(&listing, "drwxr-xr-x 2 owner group 4096 Jan 02 10:00 %s\n", baseOf(path))
				}
			}
			f.mu.Unlock()
			_, _ = data.Write([]byte(listing.String()))
			_ = data.Close()
			write("226 transfer complete")
		case "RETR":
			f.mu.Lock()
			content, ok := f.files[strings.Trim(resolve(cwd, arg), "/")]
			f.mu.Unlock()
			if !ok {
				write("550 File not found")
				continue
			}
			write("150 opening data connection")
			accepted, err := acceptPending()
			if err != nil {
				write("425 data connection failed")
				continue
			}
			_, _ = accepted.Write(content)
			_ = accepted.Close()
			write("226 transfer complete")
		case "STOR":
			write("150 ready to receive")
			accepted, err := acceptPending()
			if err != nil {
				write("425 data connection failed")
				continue
			}
			content, _ := io.ReadAll(accepted)
			_ = accepted.Close()
			f.mu.Lock()
			cleaned := strings.Trim(resolve(cwd, arg), "/")
			f.files[cleaned] = content
			f.mkdirParents(parentOf("/" + cleaned))
			f.mu.Unlock()
			write("226 transfer complete")
		case "DELE":
			f.mu.Lock()
			cleaned := strings.Trim(resolve(cwd, arg), "/")
			_, ok := f.files[cleaned]
			if ok {
				delete(f.files, cleaned)
			}
			f.mu.Unlock()
			if !ok {
				write("550 File not found")
				continue
			}
			write("250 file deleted")
		case "MKD":
			f.mu.Lock()
			f.mkdirParents(resolve(cwd, arg))
			f.mu.Unlock()
			write(fmt.Sprintf("257 %q created", arg))
		case "RMD":
			f.mu.Lock()
			cleaned := resolve(cwd, arg)
			_, ok := f.dirs[cleaned]
			if ok {
				delete(f.dirs, cleaned)
				for path := range f.files {
					if strings.HasPrefix("/"+path, cleaned+"/") {
						delete(f.files, path)
					}
				}
				for path := range f.dirs {
					if strings.HasPrefix(path, cleaned+"/") {
						delete(f.dirs, path)
					}
				}
			}
			f.mu.Unlock()
			if !ok {
				write("550 directory not found")
				continue
			}
			write("250 directory removed")
		case "RNFR":
			renameFrom = strings.Trim(resolve(cwd, arg), "/")
			write("350 ready for destination")
		case "RNTO":
			f.mu.Lock()
			content, ok := f.files[renameFrom]
			if ok {
				delete(f.files, renameFrom)
				destination := strings.Trim(resolve(cwd, arg), "/")
				f.files[destination] = content
				f.mkdirParents(parentOf("/" + destination))
			}
			f.mu.Unlock()
			if !ok {
				write("550 file not found")
				continue
			}
			write("250 rename complete")
		case "QUIT":
			write("221 bye")
			return
		default:
			write("502 command not implemented")
		}
	}
}

// pendingDataListener carries the EPSV listener between the EPSV reply and
// the transfer command. Tests run sequentially so a package-level variable
// keyed per server keeps the fake simple.
var pendingDataListeners sync.Map

func acceptPending() (net.Conn, error) {
	value, ok := pendingDataListeners.LoadAndDelete("current")
	if !ok {
		return nil, fmt.Errorf("no pending data listener")
	}
	listener := value.(net.Listener)
	accepted, err := listener.Accept()
	_ = listener.Close()
	return accepted, err
}

func parentOf(path string) string {
	index := strings.LastIndex(path, "/")
	if index <= 0 {
		return "/"
	}
	return path[:index]
}

func baseOf(path string) string {
	parent := parentOf(path)
	if parent == "/" {
		return strings.TrimPrefix(path, "/")
	}
	return strings.TrimPrefix(path, parent+"/")
}

// resolve turns a client-supplied path (absolute, relative, "." or empty)
// into an absolute fake-server path using the session's cwd.
func resolve(cwd, arg string) string {
	trimmed := strings.TrimSpace(arg)
	if trimmed == "" || trimmed == "." {
		return cwd
	}
	if strings.HasPrefix(trimmed, "/") {
		return "/" + strings.Trim(trimmed, "/")
	}
	if cwd == "/" {
		return "/" + strings.Trim(trimmed, "/")
	}
	return cwd + "/" + strings.Trim(trimmed, "/")
}

/* ---------------- driver tests ---------------- */

func newFTPConn(t *testing.T, fake *fakeFTP, baseDir string) *ftpConn {
	t.Helper()
	item := domain.Storage{
		Name: "Backup", Driver: domain.StorageDriverFTP,
		Host: "127.0.0.1", Port: fake.port(), Username: "user", TLSMode: domain.StorageTLSNone, BaseDir: baseDir,
	}
	conn, err := dialFTP(t.Context(), item, "pass")
	if err != nil {
		t.Fatalf("dialFTP() error = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn.(*ftpConn)
}

func TestFTPProbe(t *testing.T) {
	fake := newFakeFTP(t)
	conn := newFTPConn(t, fake, "")
	if err := conn.Probe(t.Context()); err != nil {
		t.Fatalf("Probe() error = %v", err)
	}
}

func TestFTPListFoldersFirst(t *testing.T) {
	fake := newFakeFTP(t)
	fake.seedFile("zeta.txt", "z")
	fake.seedFile("reports/2026/c.csv", "ccc")
	fake.seedDir("media")
	conn := newFTPConn(t, fake, "")

	entries, err := conn.List(t.Context(), "")
	if err != nil {
		t.Fatalf("List(\"\") error = %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("List(\"\") = %#v", entries)
	}
	if !entries[0].IsDir || entries[0].Name != "media" {
		t.Errorf("entries[0] = %#v, want media dir", entries[0])
	}
	if !entries[1].IsDir || entries[1].Name != "reports" {
		t.Errorf("entries[1] = %#v, want reports dir", entries[1])
	}
	if entries[2].IsDir || entries[2].Name != "zeta.txt" || entries[2].Size != 1 {
		t.Errorf("entries[2] = %#v, want zeta.txt file", entries[2])
	}

	entries, err = conn.List(t.Context(), "reports/2026")
	if err != nil {
		t.Fatalf("List(reports/2026) error = %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "c.csv" || entries[0].Size != 3 {
		t.Fatalf("List(reports/2026) = %#v", entries)
	}
}

func TestFTPUploadAndDownload(t *testing.T) {
	fake := newFakeFTP(t)
	conn := newFTPConn(t, fake, "")

	local := t.TempDir() + "/chart.png"
	if err := writeFileLocal(local, []byte("PNGDATA")); err != nil {
		t.Fatal(err)
	}
	uploaded, err := conn.UploadFile(t.Context(), local, "reports/", "")
	if err != nil {
		t.Fatalf("UploadFile() error = %v", err)
	}
	if uploaded.Key != "reports/chart.png" || uploaded.Size != 7 || uploaded.Driver != "ftp" {
		t.Fatalf("UploadFile() = %#v", uploaded)
	}

	data, err := conn.UploadData(t.Context(), []byte("hello"), "notes/greeting.txt", "text/plain")
	if err != nil {
		t.Fatalf("UploadData() error = %v", err)
	}
	if data.Key != "notes/greeting.txt" || data.Size != 5 {
		t.Fatalf("UploadData() = %#v", data)
	}

	downloaded, err := conn.Download(t.Context(), "notes/greeting.txt", t.TempDir()+"/dl/")
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	if downloaded.Name != "greeting.txt" || downloaded.Bytes != 5 {
		t.Fatalf("Download() = %#v", downloaded)
	}
	content, err := readFileLocal(downloaded.Path)
	if err != nil || string(content) != "hello" {
		t.Fatalf("downloaded content = %q, %v", content, err)
	}
}

func TestFTPDownloadMissing(t *testing.T) {
	fake := newFakeFTP(t)
	conn := newFTPConn(t, fake, "")
	if _, err := conn.Download(t.Context(), "nope.txt", t.TempDir()+"/nope.txt"); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("Download(missing) error = %v", err)
	}
}

func TestFTPDeleteFileAndFolder(t *testing.T) {
	fake := newFakeFTP(t)
	fake.seedFile("logs/app.log", "line")
	fake.seedFile("logs/old/1.log", "1")
	fake.seedFile("logs/old/2.log", "2")
	fake.seedFile("readme.md", "hi")
	conn := newFTPConn(t, fake, "")

	result, err := conn.Delete(t.Context(), "readme.md", false)
	if err != nil || !result.Deleted || result.Count != 1 {
		t.Fatalf("Delete(file) = %#v, %v", result, err)
	}
	if _, err := conn.Delete(t.Context(), "logs", false); err == nil || !strings.Contains(err.Error(), "recursive") {
		t.Fatalf("Delete(folder, false) error = %v", err)
	}
	result, err = conn.Delete(t.Context(), "logs", true)
	if err != nil || !result.Deleted {
		t.Fatalf("Delete(folder, true) = %#v, %v", result, err)
	}
	// logs + logs/app.log + logs/old + 1.log + 2.log = 5 entries removed
	if result.Count != 5 {
		t.Fatalf("Delete(folder, true) count = %d, want 5", result.Count)
	}
	if fake.fileCount() != 0 {
		t.Fatalf("files left: %d", fake.fileCount())
	}
	if _, err := conn.Delete(t.Context(), "ghost", false); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("Delete(missing) error = %v", err)
	}
}

func TestFTPMakeDirAndMove(t *testing.T) {
	fake := newFakeFTP(t)
	fake.seedFile("draft.csv", "a,b,c")
	conn := newFTPConn(t, fake, "")

	made, err := conn.MakeDir(t.Context(), "reports")
	if err != nil || !made.Created {
		t.Fatalf("MakeDir() = %#v, %v", made, err)
	}
	if !fake.hasDir("reports") {
		t.Fatal("reports dir missing")
	}
	moved, err := conn.Move(t.Context(), "draft.csv", "reports/final.csv")
	if err != nil || !moved.Moved {
		t.Fatalf("Move() = %#v, %v", moved, err)
	}
	if fake.hasFile("draft.csv") || !fake.hasFile("reports/final.csv") {
		t.Fatalf("move left draft=%v final=%v", fake.hasFile("draft.csv"), fake.hasFile("reports/final.csv"))
	}
	if _, err := conn.Move(t.Context(), "ghost.csv", "anywhere"); err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("Move(missing) error = %v", err)
	}
}

func TestFTPBaseDir(t *testing.T) {
	fake := newFakeFTP(t)
	fake.seedFile("srv/data/inside.txt", "x")
	fake.seedFile("outside.txt", "y")
	conn := newFTPConn(t, fake, "srv/data")

	entries, err := conn.List(t.Context(), "")
	if err != nil {
		t.Fatalf("List with base dir error = %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "inside.txt" {
		t.Fatalf("List with base dir = %#v", entries)
	}
}

func TestFTPRedialAfterClose(t *testing.T) {
	fake := newFakeFTP(t)
	fake.seedFile("a.txt", "one")
	conn := newFTPConn(t, fake, "")
	if _, err := conn.List(t.Context(), ""); err != nil {
		t.Fatalf("first List() error = %v", err)
	}
	// Simulate the server dropping the control connection.
	conn.mu.Lock()
	if conn.conn != nil {
		_ = conn.conn.Quit()
		conn.conn = nil // pretend the channel went stale without our knowing
	}
	conn.mu.Unlock()
	entries, err := conn.List(t.Context(), "")
	if err != nil {
		t.Fatalf("List() after stale connection error = %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "a.txt" {
		t.Fatalf("List() after redial = %#v", entries)
	}
}

func TestFTPIdleRedial(t *testing.T) {
	fake := newFakeFTP(t)
	fake.seedFile("a.txt", "one")
	conn := newFTPConn(t, fake, "")
	// Pretend the connection has been idle beyond the redial threshold.
	conn.mu.Lock()
	conn.lastUsed = time.Now().Add(-2 * ftpIdleRedial)
	conn.mu.Unlock()
	if err := conn.Probe(t.Context()); err != nil {
		t.Fatalf("Probe() after idle error = %v", err)
	}
}

func writeFileLocal(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}

func readFileLocal(path string) ([]byte, error) {
	return os.ReadFile(path)
}
