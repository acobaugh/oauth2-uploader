package main

import (
	"bytes"
	"github.com/Tkanos/gonfig"
	"github.com/alexflint/go-arg"
	log "github.com/sirupsen/logrus"
	lSyslog "github.com/sirupsen/logrus/hooks/syslog"
	"golang.org/x/net/context"
	"golang.org/x/oauth2/clientcredentials"
	"io"
	"log/syslog"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

type Cfg struct {
	ClientID     string
	ClientSecret string
	TokenURL     string
	ServiceURL   string
	Timeout      int
}

type Args struct {
	Config string `arg:"required"`
	Key    string `arg:"required"`
	File   string `arg:"required"`
	Syslog bool
}

func main() {
	// structured logging
	log.SetFormatter(&log.JSONFormatter{})

	// args
	args, err := parseArgs()
	if err != nil {
		log.Fatal(err)
	}

	// syslog
	if args.Syslog {
		log.Info("Enabling syslog")
		sysloghook, err := lSyslog.NewSyslogHook("", "", syslog.LOG_INFO, "")

		if err == nil {
			log.AddHook(sysloghook)
		} else {
			log.Warn(err)
		}
	}

	// read our config file
	cfg, err := parseConf(args.Config)
	if err != nil {
		log.Fatal(err)
	}

	// create context
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Timeout)*time.Second)
	defer cancel()

	// create an http client and get our oauth token
	client := oauthClient(ctx, cfg)

	// open the input file
	file, err := os.Open(args.File)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	log.WithFields(log.Fields{
		"file":        args.File,
		"service_url": cfg.ServiceURL,
	}).Info("Uploading")

	resp, err := uploadFile(client, cfg.ServiceURL, args.Key, file)
	if err != nil {
		log.Fatal(err)
	}
	log.Info(resp.Status)
}

func parseConf(cfgFile string) (Cfg, error) {
	// read config file/env
	cfg := Cfg{Timeout: 60}
	err := gonfig.GetConf(cfgFile, &cfg)

	return cfg, err
}

func parseArgs() (Args, error) {
	var args Args
	err := arg.Parse(&args)

	return args, err
}

func oauthClient(ctx context.Context, c Cfg) *http.Client {
	// oauth2 client
	oauthConf := clientcredentials.Config{
		ClientID:     c.ClientID,
		ClientSecret: c.ClientSecret,
		TokenURL:     c.TokenURL,
		Scopes:       []string{},
	}

	return oauthConf.Client(ctx)
}

func uploadFile(client *http.Client, url string, key string, file *os.File) (res *http.Response, err error) {
	var b bytes.Buffer
	var fw io.Writer
	w := multipart.NewWriter(&b)

	// create the form
	if fw, err = w.CreateFormFile(key, file.Name()); err != nil {
		return
	}

	if _, err = io.Copy(fw, file); err != nil {
		return
	}

	w.Close()

	// create a new request
	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		return
	}
	// set content-type
	req.Header.Set("Content-Type", w.FormDataContentType())

	// submit the request
	res, err = client.Do(req)

	return
}
