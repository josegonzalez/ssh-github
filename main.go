package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/gliderlabs/ssh"
	"github.com/kr/pty"
	gossh "golang.org/x/crypto/ssh"
)

var (
	CheckGithubUser = toBool(getenv("CHECK_GITHUB_USER", "false"))
	GithubUser      = os.Getenv("GITHUB_USER")
	HostKeyFile     = os.Getenv("HOST_KEY_FILE")
	IdleTimeout     = 10 * time.Minute
	Port            = getenv("PORT", "2222")
	SshEntrypoint   = getenv("SSH_ENTRYPOINT", "/bin/bash")
	SshGroupID      = os.Getenv("SSH_GROUP_ID")
	SshUserID       = os.Getenv("SSH_USER_ID")
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
	if SshGroupID != "" && SshUserID != "" {
		gid, err := strconv.ParseUint(SshGroupID, 10, 32)
		if err != nil {
			return
		}

		uid, err := strconv.ParseUint(SshUserID, 10, 32)
		if err != nil {
			return
		}

		cmd.SysProcAttr = &syscall.SysProcAttr{}
		cmd.SysProcAttr.Credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
	}

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

func fetchHostSigners(hostKeyFile string) (signers []ssh.Signer, err error) {
	if HostKeyFile == "" {
		return
	}

	for _, line := range strings.Split(HostKeyFile, ":") {
		if line == "" {
			continue
		}

		log.Println(fmt.Sprintf("parsing hostkey '%s'", line))
		pemBytes, errReadFile := ioutil.ReadFile(line)
		if errReadFile != nil {
			return signers, errReadFile
		}

		signer, errParse := gossh.ParsePrivateKey(pemBytes)
		if errParse != nil {
			return signers, errParse
		}
		signers = append(signers, signer)
	}

	return
}

func main() {
	if GithubUser == "" {
		log.Println("no GITHUB_USER specified")
		os.Exit(1)
	}

	if !validSshEntrypoint(SshEntrypoint) {
		log.Println("invalid SSH Entrypoint")
		os.Exit(1)
	}

	log.Println(fmt.Sprintf("fetching github ssh keys for %s", GithubUser))
	allowedPublicKeys, err := fetchGithubKeys(GithubUser)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	serverAddress := fmt.Sprintf(":%s", Port)

	signers, err := fetchHostSigners(HostKeyFile)
	if err != nil {
		log.Println(err)
		os.Exit(1)
	}

	server := &ssh.Server{
		Addr:        serverAddress,
		Handler:     sshHandler,
		HostSigners: signers,
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

	log.Println(fmt.Sprintf("starting %s ssh server on %s", SshEntrypoint, serverAddress))
	log.Fatal(server.ListenAndServe())
}
