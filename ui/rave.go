package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/morikuni/aec"
	"github.com/pkg/browser"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

type Rave struct {
	// Show extra details useful for debugging a desynced rave.
	ShowDetails bool

	// Auth configuration for syncing with Spotify.
	SpotifyAuth *spotifyauth.Authenticator

	// File path where tokens will be cached.
	SpotifyTokenPath string

	// the sequence to visualize
	start time.Time
	marks []spotify.Marker

	// syncing along to music
	track *spotify.FullTrack

	// refresh rate
	fps float64

	// current position in the marks sequence
	pos int
}

var colors = []aec.ANSI{
	aec.RedF,
	aec.GreenF,
	aec.YellowF,
	aec.BlueF,
	aec.MagentaF,
	aec.CyanF,
}

// DefaultBPM is a sane default of 123 beats per minute.
const DefaultBPM = 123
const FramesPerBeat = 10

func NewRave() *Rave {
	return &Rave{}
}

func (rave *Rave) Reset() {
	rave.start = time.Now()
	rave.marks = []spotify.Marker{
		{
			Start:    0,
			Duration: 60.0 / DefaultBPM,
		},
	}
	rave.fps = (DefaultBPM / 60.0) * FramesPerBeat
}

type authed struct {
	tok *oauth2.Token
}

func (model *Rave) Init() tea.Cmd {
	ctx := context.TODO()

	cmds := []tea.Cmd{}

	client, err := model.existingAuth(ctx)
	if err == nil {
		cmds = append(cmds, model.sync(ctx, client))
	} else {
		cmds = append(cmds, tea.Printf("existing auth: %s", err))
	}

	cmds = append(cmds, tick(model.fps))

	return tea.Batch(cmds...)
}

func (model *Rave) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		return model, tick(model.fps)
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Rave):
			return model, model.Sync()
		case key.Matches(msg, keys.EndRave):
			return model, model.Desync()
		case key.Matches(msg, keys.Debug):
			model.ShowDetails = !model.ShowDetails
		}
	}

	return model, nil
}

func (model *Rave) View() string {
	now := time.Now()

	pos, pct := model.Progress(now)

	var out string

	switch {
	case pos == -1:
		// no beat
		out += " "
	case pct > 0.6:
		// faded out
		out += " "
	case pct > 0.5:
		out += "░"
	case pct > 0.4:
		out += "▒"
	case pct > 0.3:
		out += "▓"
	default:
		out += "█"
	}

	if model.track != nil && pos != -1 {
		out = colors[pos%len(colors)].Apply(out)
	}

	if model.ShowDetails {
		out += model.Details(now)
	}

	return out
}

func (model *Rave) Details(now time.Time) string {
	if model.track == nil {
		return ""
	}

	pos, _ := model.Progress(now)

	var out string

	if pos%2 == 0 {
		out += "♫ "
	} else {
		out += "♪ "
	}

	for i, artist := range model.track.Artists {
		if i > 0 {
			out += ", "
		}
		out += artist.Name
	}

	out += " - " + model.track.Name

	if pos != -1 {
		mark := model.marks[pos]

		dur := time.Duration(mark.Duration * float64(time.Second))
		bpm := time.Minute / dur

		out += " "
		out += fmt.Sprintf(
			"♪ %d/%d %dbpm %.1ffps",
			pos+1,
			len(model.marks),
			bpm,
			model.fps,
		)

		out = colors[pos%len(colors)].Apply(out)
	}

	return out
}

func (sched *Rave) Progress(now time.Time) (int, float64) {
	for i := int(sched.pos); i < len(sched.marks); i++ {
		mark := sched.marks[i]

		start := sched.start.Add(time.Duration(mark.Start * float64(time.Second)))

		dur := time.Duration(mark.Duration * float64(time.Second))

		end := start.Add(dur)
		if start.Before(now) && end.After(now) {
			// found the current beat
			sched.pos = i
			return i, float64(now.Sub(start)) / float64(dur)
		}

		if start.After(now) {
			// in between two beats
			return -1, 0
		}
	}

	// reached the end of the beats; replay
	sched.start = now
	sched.pos = 0

	return -1, 0
}

func (m *Rave) spotifyAuth(ctx context.Context) (*spotify.Client, error) {
	auth := spotifyauth.New(
		spotifyauth.WithClientID("56f38795c77d45ee8d9db76a950258fc"),
		spotifyauth.WithRedirectURL("http://localhost:6507/callback"),
		spotifyauth.WithScopes(spotifyauth.ScopeUserReadCurrentlyPlaying),
	)

	if client, err := m.existingAuth(ctx); err == nil {
		// user has authenticated previously, no need for auth flow
		return client, nil
	}

	ch := make(chan *spotify.Client)

	state, err := b64rand(16)
	if err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}

	codeVerifier, err := b64rand(64)
	if err != nil {
		return nil, fmt.Errorf("generate verifier: %w", err)
	}

	codeChallenge := b64s256([]byte(codeVerifier))

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		defer close(ch)

		tok, err := auth.Token(r.Context(), state, r,
			oauth2.SetAuthURLParam("code_verifier", codeVerifier))
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to get token: %s", err), http.StatusForbidden)
			return
		}

		if st := r.FormValue("state"); st != state {
			http.Error(w, fmt.Sprintf("bad state: %s != %s", st, state), http.StatusForbidden)
			return
		}

		err = m.saveToken(tok)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to save token: %s", err), http.StatusInternalServerError)
			return
		}

		ch <- spotify.New(auth.Client(r.Context(), tok))
	})

	go http.ListenAndServe(":6507", mux)

	authURL := auth.AuthURL(state,
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
	)

	err = browser.OpenURL(authURL)
	if err != nil {
		return nil, fmt.Errorf("open browser: %w", err)
	}

	select {
	case client, ok := <-ch:
		if !ok {
			return nil, fmt.Errorf("callback error")
		}

		return client, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *Rave) existingAuth(ctx context.Context) (*spotify.Client, error) {
	if m.SpotifyAuth == nil || m.SpotifyTokenPath == "" {
		return nil, fmt.Errorf("no auth configured")
	}

	content, err := os.ReadFile(m.SpotifyTokenPath)
	if err != nil {
		return nil, err
	}

	var tok *oauth2.Token
	if err := json.Unmarshal(content, &tok); err != nil {
		return nil, err
	}

	client := spotify.New(m.SpotifyAuth.Client(ctx, tok))

	// check if the token is still valid and/or refresh
	refresh, err := client.Token()
	if err != nil {
		return nil, err
	}

	// save new token if refreshed
	if refresh.AccessToken != tok.AccessToken {
		err = m.saveToken(refresh)
		if err != nil {
			return nil, err
		}
	}

	return client, nil
}

func (m *Rave) Sync() tea.Cmd {
	ctx := context.TODO()

	client, err := m.spotifyAuth(ctx)
	if err != nil {
		return tea.Println("failed to authenticate:", err)
	}

	return m.sync(ctx, client)
}

func (m *Rave) Desync() tea.Cmd {
	if m.SpotifyTokenPath != "" {
		_ = os.Remove(m.SpotifyTokenPath)
	}

	m.track = nil

	return nil
}

func (m *Rave) sync(ctx context.Context, client *spotify.Client) tea.Cmd {
	playing, err := client.PlayerCurrentlyPlaying(ctx)
	if err != nil {
		return tea.Printf("failed to get currently playing: %s", err)
	}

	if playing.Item == nil {
		return tea.Printf("nothing playing")
	}

	analysis, err := client.GetAudioAnalysis(ctx, playing.Item.ID)
	if err != nil {
		return tea.Printf("failed to get audio analysis: %s", err)
	}

	// update the new timing
	m.marks = analysis.Beats

	// the Spotify API is strange: Timestamp doesn't do what it's documented to
	// do. the docs say it returns the server timestamp, but that's not at all
	// true. instead it reflects the last time the state was updated.
	//
	// assuming no user interaction, for the first 30 seconds Timestamp will
	// reflect the time the song started, but after 30 seconds it gets bumped
	// once again without the user doing anything. this doesn't happen for
	// every song.
	//
	// since it's going to be wrong half the time no matter what, let's just
	// use the timestamp value directly. :(
	m.start = time.UnixMilli(playing.Timestamp)

	// update the FPS appropriately for the track's average tempo
	bpm := analysis.Track.Tempo
	bps := bpm / 60.0
	fps := bps * 10 // each beat's frames are spread across tenths of a second
	m.fps = fps

	// rewind to the beginning in case the song changed
	//
	// NB: this doesn't actually reset the sequence shown to the user, it just
	// affects where we start looping through in Progress
	m.pos = 0

	// save the playing track to enable fancier UI + music status
	m.track = playing.Item

	return nil
}

func (m *Rave) saveToken(tok *oauth2.Token) error {
	if m.SpotifyTokenPath == "" {
		return nil
	}

	payload, err := json.Marshal(tok)
	if err != nil {
		return err
	}

	_ = os.WriteFile(m.SpotifyTokenPath, payload, 0600)
	if err != nil {
		return err
	}

	return nil
}
