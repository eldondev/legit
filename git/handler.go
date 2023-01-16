package git

import (
	"reflect"
	"bytes"
	"fmt"
	"log"
	"net/http"
	"time"
	"path/filepath"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/format/pktline"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp/sideband"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
	"github.com/kataras/muxie"
)

func InfoRefsHandler(root string) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		name := muxie.GetParam(rw, "name")

		service := r.URL.Query().Get("service")
		if service != "git-upload-pack" && service != "git-receive-pack" {
			http.Error(rw, "only smart git", http.StatusForbidden)
			return
		}

		rw.Header().Set("content-type", fmt.Sprintf("application/x-%s-advertisement", service))

		ep, err := transport.NewEndpoint("/")
		if err != nil {
			http.Error(rw, err.Error(), 500)
			log.Println(err)
			return
		}
		dir, err := securejoin.SecureJoin(root, name)
		if err != nil {
			http.Error(rw, err.Error(), 500)
			log.Println(err)
			return
		}
		dir = filepath.Join(dir, ".git")
		log.Printf("dir: %s", dir)
		bfs := osfs.New(dir)
		ld := server.NewFilesystemLoader(bfs)
		svr := server.NewServer(ld)

		var sess transport.Session

		if service == "git-upload-pack" {
			sess, err = svr.NewUploadPackSession(ep, nil)
			if err != nil {
				http.Error(rw, err.Error(), 500)
				log.Println(err)
				return
			}
		} else {
			sess, err = svr.NewReceivePackSession(ep, nil)
			if err != nil {
				http.Error(rw, err.Error(), 500)
				log.Println(err)
				return
			}
		}

		ar, err := sess.AdvertisedReferencesContext(r.Context())
		if err != nil {
			http.Error(rw, err.Error(), 500)
			log.Println(err)
			return
		}
		ar.Prefix = [][]byte{
			[]byte(fmt.Sprintf("# service=%s", service)),
			pktline.Flush,
		}
		if err := ar.Capabilities.Add("no-thin"); err != nil {
			http.Error(rw, err.Error(), 500)
			log.Println(err)
			return
		}
		err = ar.Encode(rw)
		if err != nil {
			http.Error(rw, err.Error(), 500)
			log.Println(err)
			return
		}
	}
}

func ReceivePackHandler(root string) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		name := muxie.GetParam(rw, "name")

		rw.Header().Set("content-type", "application/x-git-receive-pack-result")

		upr := packp.NewReferenceUpdateRequest()
		upr.Capabilities.Add("side-band-64k")
		err := upr.Decode(r.Body)
		if err != nil {
			http.Error(rw, err.Error(), 500)
			log.Println(err)
			return
		}

		ep, err := transport.NewEndpoint("/")
		if err != nil {
			http.Error(rw, err.Error(), 500)
			log.Println(err)
			return
		}
		dir, err := securejoin.SecureJoin(root, name)
		if err != nil {
			http.Error(rw, err.Error(), 500)
			log.Println(err)
			return
		}
		dir = filepath.Join(dir, ".git")
		log.Printf("dir: %s", dir)
		bfs := osfs.New(dir)
		ld := server.NewFilesystemLoader(bfs)
		svr := server.NewServer(ld)
		sess, err := svr.NewReceivePackSession(ep, nil)
		if err != nil {
			http.Error(rw, err.Error(), 500)
			log.Println(err)
			return
		}
		res, err := sess.ReceivePack(r.Context(), upr)
		if err != nil {
			http.Error(rw, err.Error(), 500)
			log.Println(err)
			return
		}
		nm := sideband.NewMuxer(sideband.Sideband64k, rw)
		for i := 20; i > 0; i-- {
			nm.WriteChannel(sideband.ProgressMessage, []byte(fmt.Sprintf("Bonk %d\n", i)))
			if fr, ok := rw.(*muxie.Writer).ResponseWriter.(http.Flusher); ok {
				fr.Flush()
			}
			time.Sleep(1 * time.Second)
		}
		var g bytes.Buffer
		res.Encode(&g)
		nm.WriteChannel(sideband.PackData, g.Bytes())
		rw.Write(pktline.FlushPkt)
		if err != nil {
			http.Error(rw, err.Error(), 500)
			log.Println(err)
			return
		}
	}
}

func UploadPackHandler(root string) http.HandlerFunc {
	return func(rw http.ResponseWriter, r *http.Request) {
		name := muxie.GetParam(rw, "name")

		rw.Header().Set("content-type", "application/x-git-upload-pack-result")

		upr := packp.NewUploadPackRequest()
		err := upr.Decode(r.Body)
		if err != nil {
			http.Error(rw, err.Error(), 500)
			log.Println(err)
			return
		}

		ep, err := transport.NewEndpoint("/")
		if err != nil {
			http.Error(rw, err.Error(), 500)
			log.Println(err)
			return
		}
		dir, err := securejoin.SecureJoin(root, name)
		if err != nil {
			http.Error(rw, err.Error(), 500)
			log.Println(err)
			return
		}
		dir = filepath.Join(dir, ".git")
		log.Printf("dir: %s", dir)
		bfs := osfs.New(dir)
		ld := server.NewFilesystemLoader(bfs)
		svr := server.NewServer(ld)
		sess, err := svr.NewUploadPackSession(ep, nil)
		if err != nil {
			http.Error(rw, err.Error(), 500)
			log.Println(err)
			return
		}
		_, err = sess.UploadPack(r.Context(), upr)
		if err != nil {
			http.Error(rw, err.Error(), 500)
			log.Println(err)
			return
		}

		if err != nil {
			http.Error(rw, err.Error(), 500)
			log.Println(err)
			return
		}
	}
}
