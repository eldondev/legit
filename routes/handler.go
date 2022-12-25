package routes

import (
	"net/http"

	"git.icyphox.sh/legit/config"
	"git.icyphox.sh/legit/git"
	"github.com/kataras/muxie"
)

func Handlers(c *config.Config) http.Handler {
	d := deps{c}

	mux := muxie.NewMux()
	mux.PathCorrection = true

	mux.HandleFunc("/", d.Index)
	mux.HandleFunc("/:name", d.RepoIndex)
	mux.HandleFunc("/static/*path", d.ServeStatic)
	mux.HandleFunc("/:name/tree/:ref/*path", d.RepoTree)
	mux.HandleFunc("/:name/blob/:ref/*path", d.FileContent)
	mux.HandleFunc("/:name/log/:ref", d.Log)
	mux.HandleFunc("/:name/commit/:ref", d.Diff)
	mux.HandleFunc("/:name/refs", d.Refs)

	mux.HandleFunc("/:name/info/refs", git.InfoRefsHandler(c.Repo.ScanPath))
	mux.HandleFunc("/:name/git-upload-pack", git.UploadPackHandler(c.Repo.ScanPath))
	mux.HandleFunc("/:name/git-receive-pack", git.ReceivePackHandler(c.Repo.ScanPath))

	return mux
}
