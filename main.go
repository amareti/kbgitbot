package main

import (
	"encoding/json"
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

type pusher struct {
	Name  string
	Email string
}

type repo struct {
	FullName string `json:"full_name"`
}

type commit struct {
	ID      string `json:"id"`
	Message string
}

type pushReq struct {
	Ref        string
	Pusher     pusher
	Repository repo
	Commits    []commit
}

func (s *BotServer) handlePushReq(body string) (res string, err error) {
	var pr pushReq
	if err = json.Unmarshal([]byte(body), &pr); err != nil {
		return "", err
	}
	res = fmt.Sprintf("[%s] %s pushed %d commits", pr.Repository.FullName, pr.Pusher.Name,
		len(pr.Commits))
	for _, commit := range pr.Commits {
		res += fmt.Sprintf("\\n>%s - %s", commit.ID, commit.Message)
	}
	return res, nil
}

func (s *BotServer) handlePost(w http.ResponseWriter, r *http.Request) {
	teamName := r.URL.Query().Get("team")
	if teamName == "" {
		s.debug("invalid request, no team name specified")
		return
	}

	var msg string
	var err error
	typ := r.Header.Get("X-GitHub-Event")
	body := r.FormValue("payload")
	switch typ {
	case "push":
		msg, err = s.handlePushReq(body)
	default:
		err = fmt.Errorf("unknown event type: %s", typ)
	}
	if err != nil {
		s.debug("error handling hook event: %s", err.Error())
		return
	}

	/*decoder := json.NewDecoder(r.Body)
	var t pushReq
	err := decoder.Decode(&t)
	if err != nil {
		s.debug("unable to parse
	}
	defer req.Body.Close()
	log.Println(t.Test)*/
	if err := s.kbc.SendMessageByTeamName(teamName, msg, &s.opts.Channel); err != nil {
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
