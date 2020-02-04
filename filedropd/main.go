package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/foxcpp/filedrop"
	"gopkg.in/yaml.v2"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage:", os.Args[0], "<config file>")
		os.Exit(1)
	}

	confBlob, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		log.Fatalln("Failed to read config file:", err)
	}

	config := filedrop.Config{}
	if err := yaml.Unmarshal(confBlob, &config); err != nil {
		log.Fatalln("Failed to parse config file:", err)
	}

	serv, err := filedrop.New(config)
	if err != nil {
		log.Fatalln("Failed to start server:", err)
	}

	go func() {
		log.Println("Listening on", config.ListenOn+"...")
		if err := http.ListenAndServe(config.ListenOn, serv); err != nil {
			log.Println("Failed to listen:", err)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	<-sig
}
