//go:build test

// Command mattermost_stub is the server for the manual Mattermost check of
// task 0083: the production job API, status response, and mention
// neutralization of internal/server, with the article writer replaced by one
// that returns a fixed article holding every '@' form the check pastes. It
// calls no LLM API, so the check costs nothing and gives the same article
// every time.
//
//	go build -tags test -o <binary> ./docs/tasks/0083_remove_slack_webhook/_checks/mattermost_stub/ \
//		&& <binary> -user <a real user name of the Mattermost test channel>
//
// It reads the server's configuration file only for the socket path, so it
// listens where the native messaging host connects, and it takes the same
// start lock: stop yt2column-server before starting it. The video title the
// viewer's heading shows comes from the YouTube page, not from here.
//
// This program is a one-off check of the task; make test and CI do not run it.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/isseis/yt2column/internal/config"
	"github.com/isseis/yt2column/internal/server"
	"github.com/isseis/yt2column/internal/serversock"
	"github.com/isseis/yt2column/internal/signalrun"
	"github.com/isseis/yt2column/internal/transcript"
	"github.com/isseis/yt2column/internal/writer"
)

// userNamePattern is the Mattermost user name form: lowercase letters,
// digits, '.', '-', and '_', starting with a letter.
var userNamePattern = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,21}$`)

func main() {
	signal.Ignore(syscall.SIGPIPE)
	os.Exit(signalrun.Run(func(ctx context.Context) int {
		return run(ctx, os.Args[1:], os.Stderr)
	}))
}

func run(ctx context.Context, args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("mattermost_stub", flag.ContinueOnError)
	flags.SetOutput(stderr)
	user := flags.String("user", "", "a real user name of the Mattermost test channel, without the '@'")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if flags.NArg() != 0 || !userNamePattern.MatchString(*user) {
		fmt.Fprintln(stderr, "mattermost_stub: -user takes a Mattermost user name (lowercase letters, digits, '.', '-', '_'), and nothing else is accepted")
		return 2
	}

	cfg, err := config.LoadServer(os.LookupEnv)
	if err != nil {
		fmt.Fprintf(stderr, "mattermost_stub: configuration: %v\n", err)
		return 2
	}
	srv, err := server.New(server.Options{Writer: fixedWriter{article: fixedArticle(*user)}, LogOutput: stderr})
	if err != nil {
		fmt.Fprintf(stderr, "mattermost_stub: %v\n", err)
		return 1
	}
	lock, err := serversock.Acquire(cfg.SocketPath())
	if err != nil {
		fmt.Fprintf(stderr, "mattermost_stub: %v (stop yt2column-server first)\n", err)
		return 1
	}
	defer func() {
		if err := lock.Release(); err != nil {
			fmt.Fprintf(stderr, "mattermost_stub: release the start lock: %v\n", err)
		}
	}()
	listener, err := lock.Listen()
	if err != nil {
		fmt.Fprintf(stderr, "mattermost_stub: %v\n", err)
		return 1
	}
	fmt.Fprintf(stderr, "mattermost_stub: listening on %s with the fixed article for @%s; stop with Ctrl-C\n", cfg.SocketPath(), *user)
	if err := srv.Serve(ctx, listener); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(stderr, "mattermost_stub: serving failed: %v\n", err)
		return 1
	}
	return 0
}

// fixedWriter is the article writer of the check: every transcript gets the
// same article, with the source URL of the transcript sent.
type fixedWriter struct {
	article writer.Article
}

func (w fixedWriter) Write(ctx context.Context, t transcript.Transcript) (writer.Article, error) {
	return w.WriteWithProgress(ctx, t, nil)
}

// WriteWithProgress reports the stages as a run with the post-generation
// steps turned off does, so the popup shows a finished generation.
func (w fixedWriter) WriteWithProgress(_ context.Context, t transcript.Transcript, observer writer.ProgressObserver) (writer.Article, error) {
	if observer != nil {
		observer.StageStarted(writer.StageGeneration)
		observer.StageFinished(writer.StageGeneration)
		observer.StageSkipped(writer.StageShorten)
		observer.StageSkipped(writer.StageReview)
	}
	a := w.article
	a.SourceURL = t.VideoURL
	return a, nil
}

// fixedArticle returns the article of the check. The body holds the forms
// the requirements list for the manual Mattermost check and those of the
// architecture's section 6.3 (U1 to U6, R1, R2, S1 to S3, and C), one
// paragraph each, labeled by its symbol. The
// URL path that must stay whole comes first: section 3.3's condition 5 counts
// every '[' before a URL in the whole Markdown, so it sits before U4 and S3,
// the only paragraphs with a '['.
func fixedArticle(user string) writer.Article {
	mentions := "@channel @here @" + user + " AC/@" + user
	body := []string{
		"Body: " + mentions,
		"URL path (stays a link): https://example.com/@" + user,
		"URLs: https://@channel and https://example.com/?@here",
		"Code span: `@here`",
		"```text\nFenced code: @here\n```",
		"R2: &#64;channel @&#99;hannel @\\_" + user,
		"R1: \\@channel &#97;@here",
		"S1: @**channel** `@`here",
		"S2: `x`https://example.com/@here a*https://example.com/@here* x<https://example.com/@here>",
		"U1: xhttps://example.com/@here",
		"U2: https://a_b.example.com/@here",
		"U3: https://example.com<b/@here",
		"U5: `https://example.com`/@here",
		// The second form of U6 is "参照:" (reference) before the URL.
		"U6: :https://example.com/@here 参照:https://example.com/@here",
		"~~~ a`b\nC: @here\n~~~",
		"U4: [https://example.com/@here]",
		"S3:\n\n- [x\n\nhttps://example.com/@here",
	}
	return writer.Article{
		Title:       "Mention check " + mentions,
		ChannelName: "Channel " + mentions,
		Body:        strings.Join(body, "\n\n") + "\n",
		Model:       "fixed-article",
	}
}
