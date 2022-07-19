package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/adrg/xdg"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/morikuni/aec"
	"github.com/pkg/browser"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

type Rave struct {
	Start time.Time
	Marks []spotify.Marker

	// syncing along to music
	track *spotify.FullTrack

	// refresh rate
	fps int

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

func (model *Rave) Init() tea.Cmd {
	return tick(model.fps)
}

func (model *Rave) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg.(type) {
	case tickMsg:
		return model, tick(model.fps)
	}

	return model, nil
}

func (model *Rave) View() string {
	now := time.Now()

	pos, pct := model.Progress(now)

	var symbol string
	switch {
	case pos == -1:
		// no beat
		symbol = " "
	case pct > 0.6:
		// faded out
		symbol = " "
	case pct > 0.5:
		symbol = "░"
	case pct > 0.4:
		symbol = "▒"
	case pct > 0.3:
		symbol = "▓"
	default:
		symbol = "█"
	}

	symbol = strings.Repeat(symbol, 2)

	var out string
	if model.track == nil || pos == -1 {
		out = symbol
	} else {
		out = colors[pos%len(colors)].Apply(symbol)
	}

	return out
}

func (model *Rave) Details(now time.Time) string {
	if model.track == nil {
		return ""
	}

	pos, pct := model.Progress(now)

	var out string
	if pos%2 == 0 {
		out += "♫♪ "
	} else {
		out += "♪♫ "
	}

	for i, artist := range model.track.Artists {
		if i > 0 {
			out += ", "
		}
		out += artist.Name
	}

	out += " - " + model.track.Name

	if pos != -1 {
		mark := model.Marks[pos]

		dur := time.Duration(mark.Duration * float64(time.Second))
		bpm := time.Minute / dur

		out += " "
		out += fmt.Sprintf(
			"♪ %d/%d %03d%% %dbpm %dfps",
			pos+1,
			len(model.Marks),
			int(pct*100),
			bpm,
			model.fps,
		)

		out = colors[pos%len(colors)].Apply(out)
	}

	return out
}

func (sched *Rave) Progress(now time.Time) (int, float64) {
	for i := int(sched.pos); i < len(sched.Marks); i++ {
		mark := sched.Marks[i]

		start := sched.Start.Add(time.Duration(mark.Start * float64(time.Second)))

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
	sched.Start = now
	sched.pos = 0

	return -1, 0
}

func (m *Rave) spotifyAuth(ctx context.Context) (*spotify.Client, error) {
	auth := spotifyauth.New(
		spotifyauth.WithClientID("56f38795c77d45ee8d9db76a950258fc"),
		spotifyauth.WithRedirectURL("http://localhost:6507/callback"),
		spotifyauth.WithScopes(spotifyauth.ScopeUserReadCurrentlyPlaying),
	)

	tokenPath, err := xdg.ConfigFile("bass/spotify-token")
	if err != nil {
		return nil, err
	}

	var tok *oauth2.Token
	if content, err := os.ReadFile(tokenPath); err == nil {
		err := json.Unmarshal(content, &tok)
		if err != nil {
			return nil, err
		}

		client := spotify.New(auth.Client(ctx, tok))

		// check if the token is still valid and/or refresh
		refresh, err := client.Token()
		if err != nil {
			return nil, err
		}

		// save new token if refreshed
		if refresh.AccessToken != tok.AccessToken {
			err = m.saveToken(tokenPath, refresh)
			if err != nil {
				return nil, err
			}
		}

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

		err = m.saveToken(tokenPath, tok)
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

func (m *Rave) Sync() tea.Cmd {
	ctx := context.TODO()

	client, err := m.spotifyAuth(ctx)
	if err != nil {
		return tea.Println("failed to authenticate:", err)
	}

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
	m.Marks = analysis.Beats

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
	m.Start = time.UnixMilli(playing.Timestamp)

	// update the FPS appropriately for the track's average tempo
	bpm := analysis.Track.Tempo
	bps := bpm / 60.0
	fps := bps * 10 // each beat's frames are spread across tenths of a second
	m.fps = int(fps)

	// rewind to the beginning in case the song changed
	//
	// NB: this doesn't actually reset the sequence shown to the user, it just
	// affects where we start looping through in Progress
	m.pos = 0

	// save the playing track to enable fancier UI + music status
	m.track = playing.Item

	return nil
}

func (m *Rave) saveToken(tokenPath string, tok *oauth2.Token) error {
	payload, err := json.Marshal(tok)
	if err != nil {
		return err
	}

	_ = os.WriteFile(tokenPath, payload, 0600)
	if err != nil {
		return err
	}

	return nil
}
