module git.icyphox.sh/legit

go 1.19

require (
	github.com/anmitsu/go-shlex v0.0.0-20200514113438-38f4b401e2be
	github.com/bluekeyes/go-gitdiff v0.7.0
	github.com/cyphar/filepath-securejoin v0.2.3
	github.com/dustin/go-humanize v1.0.0
	github.com/go-git/go-billy/v5 v5.4.0
	github.com/go-git/go-git/v5 v5.5.1
	github.com/kataras/muxie v1.1.2
	github.com/microcosm-cc/bluemonday v1.0.21
	github.com/russross/blackfriday/v2 v2.1.0
	golang.org/x/crypto v0.4.0
	gopkg.in/yaml.v3 v3.0.0
)

replace github.com/go-git/go-git/v5 => github.com/eldondev/go-git/v5 v5.5.3-0.20230116042059-01d464093607

require (
	github.com/Microsoft/go-winio v0.6.0 // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20221026131551-cf6655e29de4 // indirect
	github.com/acomagu/bufpipe v1.0.3 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/cloudflare/circl v1.1.0 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/go-git/gcfg v1.5.0 // indirect
	github.com/gorilla/css v1.0.0 // indirect
	github.com/imdario/mergo v0.3.13 // indirect
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/kevinburke/ssh_config v1.2.0 // indirect
	github.com/pjbgf/sha1cd v0.2.3 // indirect
	github.com/sergi/go-diff v1.1.0 // indirect
	github.com/skeema/knownhosts v1.1.0 // indirect
	github.com/xanzy/ssh-agent v0.3.3 // indirect
	golang.org/x/mod v0.7.0 // indirect
	golang.org/x/net v0.4.0 // indirect
	golang.org/x/sys v0.3.0 // indirect
	golang.org/x/tools v0.4.0 // indirect
	gopkg.in/warnings.v0 v0.1.2 // indirect
)

exclude github.com/sergi/go-diff v1.2.0
