package git

import (
	"fmt"
	"log"
	"net/http"
	"path/filepath"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-git/v5/plumbing/format/pktline"
	"github.com/go-git/go-git/v5/plumbing/protocol/packp"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/server"
	"github.com/julienschmidt/httprouter"
	"go.mills.io/router"
)

type gitHTTPHandler struct {
	root string
	*router.Router
}

func (h *gitHTTPHandler) setupRoutes() {
	if h.Router == nil {
		h.Router = router.New()
		h.GET("/:repo/info/refs", h.infoRefsHandler)
		h.POST("/:repo/git-upload-pack", h.uploadPackHandler)
		h.POST("/:repo/git-receive-pack", h.receivePackHandler)
	}
}

func (h *gitHTTPHandler) infoRefsHandler(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
	repo := p.ByName("repo")

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
	dir, err := securejoin.SecureJoin(h.root, repo)
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
	err = ar.Encode(rw)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		log.Println(err)
		return
	}
}

func (h *gitHTTPHandler) receivePackHandler(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
	repo := p.ByName("repo")

	rw.Header().Set("content-type", "application/x-git-receive-pack-result")

	upr := packp.NewReferenceUpdateRequest()
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
	dir, err := securejoin.SecureJoin(h.root, repo)
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
	defer sess.Close()
	res, err := sess.ReceivePack(r.Context(), upr)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		log.Println(err)
		return
	}

	err = res.Encode(rw)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		log.Println(err)
		return
	}
}

func (h *gitHTTPHandler) uploadPackHandler(rw http.ResponseWriter, r *http.Request, p httprouter.Params) {
	repo := p.ByName("repo")

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
	dir, err := securejoin.SecureJoin(h.root, repo)
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
	res, err := sess.UploadPack(r.Context(), upr)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		log.Println(err)
		return
	}

	err = res.Encode(rw)
	if err != nil {
		http.Error(rw, err.Error(), 500)
		log.Println(err)
		return
	}
}

func Handler(root string) http.Handler {
	h := &gitHTTPHandler{root: root}
	h.setupRoutes()
	return h
}
