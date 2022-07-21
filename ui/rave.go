package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fogleman/ease"
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

	// The animation to display.
	Frames [FramesPerBeat]string

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

// FramesPerBeat determines the granularity that the spinner's animation timing
// for each beat, i.e. 10 for tenths of a second.
const FramesPerBeat = 10

var MeterFrames = [FramesPerBeat]string{
	0: "█",
	1: "█",
	2: "▇",
	3: "▆",
	4: "▅",
	5: "▄",
	6: "▃",
	7: "▂",
	8: "▁",
	9: " ",
}

var FadeFrames = [FramesPerBeat]string{
	0: "█",
	1: "█",
	2: "▓",
	3: "▓",
	4: "▒",
	5: "▒",
	6: "░",
	7: "░",
	8: " ",
	9: " ",
}

func NewRave() *Rave {
	r := &Rave{
		Frames: MeterFrames,
	}

	r.Reset()

	return r
}

func (rave *Rave) Reset() {
	rave.start = time.Now()
	rave.marks = []spotify.Marker{
		{
			Start:    0,
			Duration: 60.0 / DefaultBPM,
		},
	}
	rave.track = nil
	rave.fps = (DefaultBPM / 60.0) * FramesPerBeat
	rave.pos = 0
}

type authed struct {
	tok *oauth2.Token
}

func (rave *Rave) Init() tea.Cmd {
	ctx := context.TODO()
	return tea.Batch(
		tick(rave.fps),
		func() tea.Msg {
			client, err := rave.existingAuth(ctx)
			if err == nil {
				return rave.sync(ctx, client)
			}

			return nil
		},
	)
}

func (rave *Rave) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case syncedMsg:
		// update the new timing
		rave.marks = msg.analysis.Beats

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
		rave.start = time.UnixMilli(msg.playing.Timestamp)

		// update the FPS appropriately for the track's average tempo
		bpm := msg.analysis.Track.Tempo
		bps := bpm / 60.0
		fps := bps * FramesPerBeat
		fps *= 10 // decrease chance of missing a frame due to timing
		fps = 165 // TODO: my monitor
		fps *= 2
		rave.fps = fps

		// rewind to the beginning in case the song changed
		//
		// NB: this doesn't actually reset the sequence shown to the user, it just
		// affects where we start looping through in Progress
		rave.pos = 0

		// save the playing track to enable fancier UI + music status
		rave.track = msg.playing.Item

		return rave, tea.Cmd(func() tea.Msg {
			return setFpsMsg(fps)
		})

	case syncErrMsg:
		return rave, tea.Printf("sync error: %s", msg.err)

	// NB: these might be hit at an upper layer instead; in which case it will
	// *not* propagate.
	case tickMsg:
		return rave, tick(rave.fps)
	case setFpsMsg:
		rave.fps = float64(msg)
		return rave, tea.Printf("rave set fps: %.1f", rave.fps)

	// NB: these are captured and forwarded at the outer level.
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Rave):
			return rave, rave.Sync()
		case key.Matches(msg, keys.EndRave):
			return rave, rave.Desync()
		case key.Matches(msg, keys.ForwardRave):
			rave.start = rave.start.Add(100 * time.Millisecond)
			rave.pos = 0 // reset and recalculate
			return rave, nil
		case key.Matches(msg, keys.BackwardRave):
			rave.start = rave.start.Add(-100 * time.Millisecond)
			rave.pos = 0 // reset and recalculate
			return rave, nil
		case key.Matches(msg, keys.Debug):
			rave.ShowDetails = !rave.ShowDetails
			return rave, nil
		}

		return rave, nil

	default:
		return rave, nil
	}
}

func (rave *Rave) View() string {
	now := time.Now()

	pos, pct := rave.Progress(now)

	var out string

	frame := int(ease.InOutCirc(pct) * 10)

	// some animations go > 100% or <100%, so be defensive and clamp to the
	// frames since that doesn't actually make sense
	if frame < 0 {
		frame = 0
	} else if frame >= FramesPerBeat {
		frame = FramesPerBeat - 1
	}

	out += rave.Frames[frame]

	out = strings.Repeat(out, 2)

	if rave.track != nil && pos != -1 {
		out = colors[pos%len(colors)].Apply(out)
	}

	if rave.ShowDetails {
		out += " " + rave.viewDetails(now, pos)
	}

	return out
}

func (model *Rave) viewDetails(now time.Time, pos int) string {
	var out string

	if model.track != nil {
		for i, artist := range model.track.Artists {
			if i > 0 {
				out += ", "
			}
			out += artist.Name
		}

		out += " - " + model.track.Name
	}

	if pos != -1 {
		mark := model.marks[pos]

		elapsed := time.Duration(mark.Start * float64(time.Second))
		dur := time.Duration(mark.Duration * float64(time.Second))
		bpm := time.Minute / dur

		if out != "" {
			out += " "
		}

		out += fmt.Sprintf(
			"%s %dbpm %.1ffps",
			elapsed.Truncate(time.Second),
			bpm,
			model.fps,
		)

		out = aec.Faint.Apply(out)
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

	return func() tea.Msg {
		client, err := m.spotifyAuth(ctx)
		if err != nil {
			return tea.Println("failed to authenticate:", err)
		}

		return m.sync(ctx, client)
	}
}

func (m *Rave) Desync() tea.Cmd {
	if m.SpotifyTokenPath != "" {
		_ = os.Remove(m.SpotifyTokenPath)
	}

	m.Reset()

	return nil
}

type syncedMsg struct {
	playing  *spotify.CurrentlyPlaying
	analysis *spotify.AudioAnalysis
}

type syncErrMsg struct {
	err error
}

func (m *Rave) sync(ctx context.Context, client *spotify.Client) tea.Msg {
	playing, err := client.PlayerCurrentlyPlaying(ctx)
	if err != nil {
		return syncErrMsg{fmt.Errorf("get currently playing: %w", err)}
	}

	if playing.Item == nil {
		return syncErrMsg{fmt.Errorf("nothing playing")}
	}

	analysis, err := client.GetAudioAnalysis(ctx, playing.Item.ID)
	if err != nil {
		return syncErrMsg{fmt.Errorf("failed to get audio analysis: %w", err)}
	}

	return syncedMsg{
		playing:  playing,
		analysis: analysis,
	}
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
