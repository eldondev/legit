package routes

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"git.icyphox.sh/legit/config"
	"git.icyphox.sh/legit/git"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/dustin/go-humanize"
	"github.com/kataras/muxie"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
)

type deps struct {
	c *config.Config
}

func (d *deps) Index(w http.ResponseWriter, r *http.Request) {
	dirs, err := os.ReadDir(d.c.Repo.ScanPath)
	if err != nil {
		d.Write500(w)
		log.Printf("reading scan path: %s", err)
		return
	}

	type info struct {
		Name, Desc, Idle string
		d                time.Time
	}

	infos := []info{}

	for _, dir := range dirs {
		if d.isIgnored(dir.Name()) {
			continue
		}

		path := filepath.Join(d.c.Repo.ScanPath, dir.Name())
		gr, err := git.Open(path, "")
		if err != nil {
			continue
		}

		c, err := gr.LastCommit()
		if err != nil {
			d.Write500(w)
			log.Println(err)
			return
		}

		desc := getDescription(path)

		infos = append(infos, info{
			Name: dir.Name(),
			Desc: desc,
			Idle: humanize.Time(c.Author.When),
			d:    c.Author.When,
		})
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[j].d.Before(infos[i].d)
	})

	tpath := filepath.Join(d.c.Dirs.Templates, "*")
	t := template.Must(template.ParseGlob(tpath))

	data := make(map[string]interface{})
	data["meta"] = d.c.Meta
	data["info"] = infos

	if err := t.ExecuteTemplate(w, "index", data); err != nil {
		log.Println(err)
		return
	}
}

func (d *deps) RepoIndex(w http.ResponseWriter, r *http.Request) {
	name := muxie.GetParam(w, "name")
	if d.isIgnored(name) {
		d.Write404(w)
		return
	}
	name = filepath.Clean(name)
	path := filepath.Join(d.c.Repo.ScanPath, name)

	gr, err := git.Open(path, "")
	if err != nil {
		d.Write404(w)
		return
	}

	commits, err := gr.Commits()
	if err != nil {
		d.Write500(w)
		log.Println(err)
		return
	}

	var readmeContent template.HTML
	for _, readme := range d.c.Repo.Readme {
		ext := filepath.Ext(readme)
		content, _ := gr.FileContent(readme)
		if len(content) > 0 {
			switch ext {
			case ".md":
				unsafe := blackfriday.Run([]byte(content), blackfriday.WithExtensions(blackfriday.CommonExtensions))
				html := bluemonday.UGCPolicy().SanitizeBytes(unsafe)
				readmeContent = template.HTML(html)
			default:
				readmeContent = template.HTML(
					fmt.Sprintf(`<pre>%s</pre>`, content),
				)
			}
			break
		}
	}

	if len(readmeContent) <= 0 {
		log.Printf("no readme found for %s", name)
	}

	var (
		licenseContent string
		licenseType    = "Unknown"
	)
	for _, license := range d.c.Repo.License {
		licenseContent, _ = gr.FileContent(license)
		if len(licenseContent) > 0 {
			var firstLine string
			for i, c := range licenseContent {
				if c == '\n' {
					firstLine = strings.ToLower(strings.TrimSpace(licenseContent[:i]))
					break
				}
			}

			// @Todo: This should probably be more robust but works for now
			switch {
			case strings.Contains(firstLine, "mit"):
				licenseType = "MIT"
			case strings.Contains(firstLine, "public domain"):
				licenseType = "Public Domain"
			case strings.Contains(firstLine, "apache license"):
				licenseType = "Apache"
			case strings.Contains(firstLine, "gnu general public license"):
				licenseType = "GNU GPLv3"
			case strings.Contains(firstLine, "gnu affero general public license"):
				licenseType = "GNU AGPLv3"
			case strings.Contains(firstLine, "gnu lesser general public license"):
				licenseType = "GNU LGPLv3"
			case strings.Contains(firstLine, "mozilla public license"):
				licenseType = "Mozilla Public License"
			case strings.Contains(firstLine, "boost software license"):
				licenseType = "Boost Software License"
			default:
				log.Printf("unknown license %q for %s", firstLine, name)
			}

			break
		}
	}

	if len(licenseContent) <= 0 {
		log.Printf("no license found for %s", name)
	}

	mainBranch, err := gr.FindMainBranch(d.c.Repo.MainBranch)
	if err != nil {
		d.Write500(w)
		log.Println(err)
		return
	}

	tpath := filepath.Join(d.c.Dirs.Templates, "*")
	t := template.Must(template.ParseGlob(tpath))

	if len(commits) >= 3 {
		commits = commits[:3]
	}

	data := make(map[string]any)
	data["name"] = name
	data["ref"] = mainBranch
	data["readme"] = readmeContent
	data["license"] = licenseContent
	data["licensetype"] = licenseType
	data["commits"] = commits
	data["desc"] = getDescription(path)
	data["servername"] = d.c.Server.Name

	if err := t.ExecuteTemplate(w, "repo", data); err != nil {
		log.Println(err)
		return
	}
}

func (d *deps) RepoTree(w http.ResponseWriter, r *http.Request) {
	name := muxie.GetParam(w, "name")
	if d.isIgnored(name) {
		d.Write404(w)
		return
	}
	treePath := muxie.GetParam(w, "path")
	ref := muxie.GetParam(w, "ref")

	name = filepath.Clean(name)
	path := filepath.Join(d.c.Repo.ScanPath, name)
	gr, err := git.Open(path, ref)
	if err != nil {
		d.Write404(w)
		return
	}

	files, err := gr.FileTree(treePath)
	if err != nil {
		d.Write500(w)
		log.Println(err)
		return
	}

	data := make(map[string]any)
	data["name"] = name
	data["ref"] = ref
	data["parent"] = treePath
	data["desc"] = getDescription(path)

	d.listFiles(files, data, w)
}

func (d *deps) FileContent(w http.ResponseWriter, r *http.Request) {
	name := muxie.GetParam(w, "name")
	if d.isIgnored(name) {
		d.Write404(w)
		return
	}
	treePath := muxie.GetParam(w, "path")
	ref := muxie.GetParam(w, "ref")

	name = filepath.Clean(name)
	path := filepath.Join(d.c.Repo.ScanPath, name)
	gr, err := git.Open(path, ref)
	if err != nil {
		d.Write404(w)
		return
	}

	contents, _ := gr.FileContent(treePath)
	data := make(map[string]any)
	data["name"] = name
	data["ref"] = ref
	data["desc"] = getDescription(path)
	data["path"] = treePath

	d.showFile(contents, data, w)
}

func (d *deps) Log(w http.ResponseWriter, r *http.Request) {
	name := muxie.GetParam(w, "name")
	if d.isIgnored(name) {
		d.Write404(w)
		return
	}
	ref := muxie.GetParam(w, "ref")

	path := filepath.Join(d.c.Repo.ScanPath, name)
	gr, err := git.Open(path, ref)
	if err != nil {
		d.Write404(w)
		return
	}

	commits, err := gr.Commits()
	if err != nil {
		d.Write500(w)
		log.Println(err)
		return
	}

	tpath := filepath.Join(d.c.Dirs.Templates, "*")
	t := template.Must(template.ParseGlob(tpath))

	data := make(map[string]interface{})
	data["commits"] = commits
	data["meta"] = d.c.Meta
	data["name"] = name
	data["ref"] = ref
	data["desc"] = getDescription(path)

	if err := t.ExecuteTemplate(w, "log", data); err != nil {
		log.Println(err)
		return
	}
}

func (d *deps) Diff(w http.ResponseWriter, r *http.Request) {
	name := muxie.GetParam(w, "name")
	if d.isIgnored(name) {
		d.Write404(w)
		return
	}
	ref := muxie.GetParam(w, "ref")

	path := filepath.Join(d.c.Repo.ScanPath, name)
	gr, err := git.Open(path, ref)
	if err != nil {
		d.Write404(w)
		return
	}

	diff, err := gr.Diff()
	if err != nil {
		d.Write500(w)
		log.Println(err)
		return
	}

	tpath := filepath.Join(d.c.Dirs.Templates, "*")
	t := template.Must(template.ParseGlob(tpath))

	data := make(map[string]interface{})

	data["commit"] = diff.Commit
	data["stat"] = diff.Stat
	data["diff"] = diff.Diff
	data["meta"] = d.c.Meta
	data["name"] = name
	data["ref"] = ref
	data["desc"] = getDescription(path)

	if err := t.ExecuteTemplate(w, "commit", data); err != nil {
		log.Println(err)
		return
	}
}

func (d *deps) Refs(w http.ResponseWriter, r *http.Request) {
	name := muxie.GetParam(w, "name")
	if d.isIgnored(name) {
		d.Write404(w)
		return
	}

	path := filepath.Join(d.c.Repo.ScanPath, name)
	gr, err := git.Open(path, "")
	if err != nil {
		d.Write404(w)
		return
	}

	tags, err := gr.Tags()
	if err != nil {
		// Non-fatal, we *should* have at least one branch to show.
		log.Println(err)
	}

	branches, err := gr.Branches()
	if err != nil {
		log.Println(err)
		d.Write500(w)
		return
	}

	tpath := filepath.Join(d.c.Dirs.Templates, "*")
	t := template.Must(template.ParseGlob(tpath))

	data := make(map[string]interface{})

	data["meta"] = d.c.Meta
	data["name"] = name
	data["branches"] = branches
	data["tags"] = tags
	data["desc"] = getDescription(path)

	if err := t.ExecuteTemplate(w, "refs", data); err != nil {
		log.Println(err)
		return
	}
}

func (d *deps) ServeStatic(w http.ResponseWriter, r *http.Request) {
	p := muxie.GetParam(w, "path")
	fn, err := securejoin.SecureJoin(d.c.Dirs.Static, p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		log.Println(err)
		return
	}

	http.ServeFile(w, r, fn)
}
