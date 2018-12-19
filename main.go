package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/gliderlabs/ssh"
	"github.com/kr/pty"
)

var (
	CheckGithubUser = toBool(getenv("CHECK_GITHUB_USER", "false"))
	SshEntrypoint   = getenv("SSH_ENTRYPOINT", "/bin/bash")
	GithubUser      = os.Getenv("GITHUB_USER")
	IdleTimeout     = 10 * time.Minute
	Port            = getenv("PORT", "2222")
	HostKeyFile     = os.Getenv("HOST_KEY_FILE")
)

func fetchGithubKeys(user string) (publicKeys []ssh.PublicKey, err error) {
	url := fmt.Sprintf("https://github.com/%s.keys", user)
	response, err := http.Get(url)
	if err != nil {
		return
	}

	defer response.Body.Close()
	contents, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return
	}

	lines := strings.Split(string(contents), "\n")
	for _, line := range lines {
		allowed, _, _, _, _ := ssh.ParseAuthorizedKey([]byte(line))
		publicKeys = append(publicKeys, allowed)
	}

	return
}

func getenv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		value = defaultValue
	}

	return value
}

func toBool(value string) bool {
	return value == "true"
}

func setWinsize(f *os.File, w, h int) {
	syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(&struct{ h, w, x, y uint16 }{uint16(h), uint16(w), 0, 0})))
}

func sshHandler(s ssh.Session) {
	log.Println(fmt.Sprintf("connection for %s", s.User()))
	if CheckGithubUser && GithubUser != s.User() {
		log.Println(fmt.Sprintf("Unrecognized user %s", s.User()))
		io.WriteString(s, fmt.Sprintf("Unrecognized user %s\n", s.User()))
		s.Exit(1)
	}

	if !validSshEntrypoint(SshEntrypoint) {
		log.Println("Invalid SSH Entrypoint")
		io.WriteString(s, "Invalid SSH Entrypoint\n")
		s.Exit(1)
	}

	cmd := exec.Command(SshEntrypoint)
	ptyReq, winCh, isPty := s.Pty()
	if isPty {
		cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", ptyReq.Term))
		f, err := pty.Start(cmd)
		if err != nil {
			panic(err)
		}
		go func() {
			for win := range winCh {
				setWinsize(f, win.Width, win.Height)
			}
		}()
		go func() {
			io.Copy(f, s)
		}()
		io.Copy(s, f)
	} else {
		io.WriteString(s, "No PTY requested.\n")
		s.Exit(1)
	}
}

func validSshEntrypoint(sshEntrypoint string) bool {
	if _, err := os.Stat(sshEntrypoint); os.IsNotExist(err) {
		return false
	}
	return true
}

func main() {
	if GithubUser == "" {
		log.Println("No GITHUB_USER specified")
		os.Exit(1)
	}

	if !validSshEntrypoint(SshEntrypoint) {
		log.Println("Invalid SSH Entrypoint")
		os.Exit(1)
	}

	log.Println(fmt.Sprintf("fetching github ssh keys for %s", GithubUser))
	allowedPublicKeys, err := fetchGithubKeys(GithubUser)
	if err != nil {
		log.Println(fmt.Printf("%s", err))
		os.Exit(1)
	}

	serverAddress := fmt.Sprintf(":%s", Port)
	log.Println(fmt.Sprintf("starting %s ssh server on %s", SshEntrypoint, serverAddress))

	if HostKeyFile != "" {
		ssh.HostKeyFile(HostKeyFile)
	}
	server := &ssh.Server{
		Addr:        serverAddress,
		Handler:     sshHandler,
		IdleTimeout: IdleTimeout,
		PublicKeyHandler: func(ctx ssh.Context, key ssh.PublicKey) bool {
			for _, allowed := range allowedPublicKeys {
				if ssh.KeysEqual(key, allowed) {
					return true
				}
			}
			return false
		},
	}
	log.Fatal(server.ListenAndServe())
}
