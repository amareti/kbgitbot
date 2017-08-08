package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

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

type user struct {
	Name  string
	Email string
}

type repo struct {
	FullName string `json:"full_name"`
}

type commit struct {
	ID        string `json:"id"`
	Message   string
	Author    user
	Committer user
}

type pushReq struct {
	Ref        string
	Deleted    bool
	Pusher     user
	Repository repo
	Commits    []commit
}

func (s *BotServer) handlePushReq(body string) (res string, err error) {
	var pr pushReq
	if err = json.Unmarshal([]byte(body), &pr); err != nil {
		return "", err
	}
	branch := strings.TrimPrefix(pr.Ref, "refs/heads/")
	if len(pr.Commits) == 0 {
		res = fmt.Sprintf("*github*\n[%s] _%s_ deleted branch `%s`", pr.Repository.FullName,
			pr.Pusher.Name, branch)
	} else {
		res = fmt.Sprintf("*github*\n[%s] _%s_ pushed %d commits to `%s`", pr.Repository.FullName,
			pr.Pusher.Name, len(pr.Commits), branch)
		for _, commit := range pr.Commits {
			toks := strings.Split(commit.Message, "\n")
			res += fmt.Sprintf("\n>`%s` %s - %s", commit.ID[0:8], toks[0], commit.Author.Name)
		}
	}
	s.debug("msg: %s", res)
	return res, nil
}

type issueUser struct {
	Login string
}

type issue struct {
	URL    string `json:"html_url"`
	Title  string
	Body   string
	User   issueUser
	Number int
}

type issueReq struct {
	Action     string
	Issue      issue
	Repository repo
}

func (s *BotServer) handleIssueReq(body string) (res string, err error) {
	var ir issueReq
	if err = json.Unmarshal([]byte(body), &ir); err != nil {
		return "", err
	}
	if ir.Action != "opened" {
		return "", errors.New("not an open issue event")
	}
	res = fmt.Sprintf(`*github*
[%s] Issue created by _%s_
>*[#%d] %s*
>%s
>%s`, ir.Repository.FullName, ir.Issue.User.Login, ir.Issue.Number, ir.Issue.Title, ir.Issue.URL,
		strings.Replace(ir.Issue.Body, "\n", "\n>", -1))
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
	case "issues":
		msg, err = s.handleIssueReq(body)
	default:
		err = fmt.Errorf("unknown event type: %s", typ)
	}
	if err != nil {
		s.debug("error handling hook event: %s", err.Error())
		return
	}

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
	flag.IntVar(&opts.ListenPort, "port", 8080, "listen port")
	flag.Parse()

	bs := NewBotServer(opts)
	if err := bs.Start(); err != nil {
		fmt.Printf("error running chat loop: %s\n", err.Error())
	}

	return 0
}
