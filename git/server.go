package git

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"log"
	"net"
	"path/filepath"

	"github.com/anmitsu/go-shlex"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
	"golang.org/x/crypto/ssh"
)

func ListenAndServe(root, addr string) error {
	config := &ssh.ServerConfig{
		NoClientAuth: true,
	}
	_, edSigner, _ := ed25519.GenerateKey(rand.Reader)
	sshSigner, _ := ssh.NewSignerFromSigner(edSigner)
	config.AddHostKey(sshSigner)

	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer lis.Close()
	for {
		conn, err := lis.Accept()
		if err != nil {
			return err
		}

		go func(conn net.Conn) {
			defer conn.Close()

			sshConn, chanc, reqc, err := ssh.NewServerConn(conn, config)
			if err != nil {
				log.Println(err)
				return
			}
			defer sshConn.Close()
			go ssh.DiscardRequests(reqc)
			for chanr := range chanc {
				switch chanr.ChannelType() {
				case "session":
					ch, reqc, err := chanr.Accept()
					if err != nil {
						log.Println(err)
						return
					}
					handleSSHSession(root, ch, reqc)
				default:
					log.Printf("unhandled channel: %s", chanr.ChannelType())
				}
			}
		}(conn)
	}
}

func handleSSHSession(root string, ch ssh.Channel, reqc <-chan *ssh.Request) {
	defer ch.Close()

	var exitCode uint32
	defer func() {
		b := ssh.Marshal(struct{ Value uint32 }{exitCode})
		ch.SendRequest("exit-status", false, b)
	}()

	envs := make(map[string]string)
	for req := range reqc {
		switch req.Type {
		case "env":
			payload := struct{ Key, Value string }{}
			ssh.Unmarshal(req.Payload, &payload)
			envs[payload.Key] = payload.Value
			req.Reply(true, nil)
		case "exec":
			payload := struct{ Value string }{}
			ssh.Unmarshal(req.Payload, &payload)
			args, err := shlex.Split(payload.Value, true)
			if err != nil {
				log.Println("lex args", err)
				exitCode = 1
				return
			}
			log.Printf("args: #%v", args)

			cmd := args[0]
			switch cmd {
			case "git-upload-pack": // read
				if gp := envs["GIT_PROTOCOL"]; gp != "version=2" {
					log.Println("unhandled GIT_PROTOCOL", gp)
					exitCode = 1
					return
				}

				dir, err := securejoin.SecureJoin(root, args[1])
				if err != nil {
					log.Println("invalid repo", args[1])
					exitCode = 1
					return
				}
				dir = filepath.Join(dir, ".git")
				log.Printf("dir: %s", dir)

				err = handleUploadPack(dir, ch)
				if err != nil {
					log.Println(err)
					exitCode = 1
					return
				}

				req.Reply(true, nil)
				return
			case "git-receive-pack": // write
				dir, err := securejoin.SecureJoin(root, args[1])
				if err != nil {
					log.Println("invalid repo", args[1])
					exitCode = 1
					return
				}
				dir = filepath.Join(dir, ".git")
				log.Printf("dir: %s", dir)

				err = handleReceivePack(dir, ch)
				if err != nil {
					log.Println(err)
					exitCode = 1
					return
				}

				req.Reply(true, nil)
				return
			default:
				log.Printf("unhandled cmd: %s", cmd)
				req.Reply(false, nil)
				exitCode = 1
				return
			}
		case "auth-agent-req@openssh.com":
			if req.WantReply {
				req.Reply(true, nil)
			}
		default:
			log.Printf("unhandled req type: %s", req.Type)
			req.Reply(false, nil)
			exitCode = 1
			return
		}
	}
}

func handleReceivePack(dir string, ch ssh.Channel) error {
	ctx := context.Background()

	ep, err := transport.NewEndpoint("/")
	if err != nil {
		return fmt.Errorf("create transport endpoint: %w", err)
	}
	bfs := osfs.New(dir)
	ld := server.NewFilesystemLoader(bfs)
	svr := server.NewServer(ld)
	sess, err := svr.NewReceivePackSession(ep, nil)
	if err != nil {
		return fmt.Errorf("create receive-pack session: %w", err)
	}

	ar, err := sess.AdvertisedReferencesContext(ctx)
	if err != nil {
		return fmt.Errorf("get advertised references: %w", err)
	}
	err = ar.Encode(ch)
	if err != nil {
		return fmt.Errorf("encode advertised references: %w", err)
	}

	rur := packp.NewReferenceUpdateRequest()
	err = rur.Decode(ch)
	if err != nil {
		return fmt.Errorf("decode reference-update request: %w", err)
	}

	res, err := sess.ReceivePack(ctx, rur)
	if err != nil {
		return fmt.Errorf("create receive-pack response: %w", err)
	}
	err = res.Encode(ch)
	if err != nil {
		return fmt.Errorf("encode receive-pack response: %w", err)
	}

	return nil
}

func handleUploadPack(dir string, ch ssh.Channel) error {
	ctx := context.Background()

	ep, err := transport.NewEndpoint("/")
	if err != nil {
		return fmt.Errorf("create transport endpoint: %w", err)
	}
	bfs := osfs.New(dir)
	ld := server.NewFilesystemLoader(bfs)
	svr := server.NewServer(ld)
	sess, err := svr.NewUploadPackSession(ep, nil)
	if err != nil {
		return fmt.Errorf("create upload-pack session: %w", err)
	}

	ar, err := sess.AdvertisedReferencesContext(ctx)
	if err != nil {
		return fmt.Errorf("get advertised references: %w", err)
	}
	if err := ar.Capabilities.Add("no-thin"); err != nil {
		return fmt.Errorf("set advertised capabilities: %w", err)
	}
	err = ar.Encode(ch)
	if err != nil {
		return fmt.Errorf("encode advertised references: %w", err)
	}

	upr := packp.NewUploadPackRequest()
	err = upr.Decode(ch)
	if err != nil {
		return fmt.Errorf("decode upload-pack request: %w", err)
	}

	res, err := sess.UploadPack(ctx, upr)
	if err != nil {
		return fmt.Errorf("create upload-pack response: %w", err)
	}
	err = res.Encode(ch)
	if err != nil {
		return fmt.Errorf("encode upload-pack response: %w", err)
	}

	return nil
}
