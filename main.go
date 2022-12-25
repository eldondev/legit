package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"git.icyphox.sh/legit/config"
	"git.icyphox.sh/legit/git"
	"git.icyphox.sh/legit/routes"
)

func main() {
	var cfg string
	flag.StringVar(&cfg, "config", "./config.yaml", "path to config file")
	flag.Parse()

	c, err := config.Read(cfg)
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		addr := fmt.Sprintf("%s:%d", c.Server.Host, c.Server.SSHPort)
		log.Println("starting SSH server on", addr)
		log.Fatal(git.ListenAndServe(c.Repo.ScanPath, addr))
	}()

	mux := routes.Handlers(c)
	addr := fmt.Sprintf("%s:%d", c.Server.Host, c.Server.HTTPPort)
	log.Println("starting HTTP server on", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
