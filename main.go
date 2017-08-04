package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
)

func main() {
	rc := mainInner()
	os.Exit(rc)
}

type Options struct {
	KeybaseLocation string
	ListenPort      int
	Channel         string
}

type BotServer struct {
	opts Options
	kbc  *kbchat.API
}

func NewBotServer(opts Options) *BotServer {
	return &BotServer{
		opts: opts,
	}
}

func (s *BotServer) debug(msg string, args ...interface{}) {
	fmt.Printf("BotServer: "+msg+"\n", args...)
}

func (s *BotServer) handlePost(w http.ResponseWriter, r *http.Request) {
	teamName := r.URL.Query().Get("team")
	if teamName == "" {
		s.debug("invalid request, no team name specified")
		return
	}
	if err := s.kbc.SendMessageByTeamName(teamName, "HI", &s.opts.Channel); err != nil {
		s.debug("failed to send message: %s", err.Error())
	}
}

func (s *BotServer) Start() (err error) {

	// Start up KB chat
	if s.kbc, err = kbchat.Start(s.opts.KeybaseLocation); err != nil {
		return err
	}

	// Start up HTTP interface
	http.HandleFunc("/", s.handlePost)
	return http.ListenAndServe(fmt.Sprintf(":%d", s.opts.ListenPort), nil)
}

func mainInner() int {
	var opts Options

	flag.StringVar(&opts.KeybaseLocation, "keybase", "keybase", "keybase command")
	flag.StringVar(&opts.Channel, "channel", "github", "channel to send messages")
	flag.IntVar(&opts.ListenPort, "port", 80, "listen port")
	flag.Parse()

	bs := NewBotServer(opts)
	if err := bs.Start(); err != nil {
		fmt.Printf("error running chat loop: %s\n", err.Error())
	}

	return 0
}
