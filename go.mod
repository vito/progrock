module github.com/vito/progrock

go 1.18

require (
	dagger.io/dagger v0.7.2
	github.com/charmbracelet/bubbles v0.16.1
	github.com/charmbracelet/bubbletea v0.24.1
	github.com/charmbracelet/lipgloss v0.7.1
	github.com/dagger/dagger v0.0.0-00010101000000-000000000000
	github.com/docker/go-units v0.5.0
	github.com/fogleman/ease v0.0.0-20170301025033-8da417bf1776
	github.com/jonboulle/clockwork v0.4.0
	github.com/muesli/termenv v0.15.1
	github.com/opencontainers/go-digest v1.0.0
	github.com/pkg/browser v0.0.0-20210911075715-681adbf594b8
	github.com/sebdah/goldie/v2 v2.5.3
	github.com/stretchr/testify v1.8.3
	github.com/vito/vt100 v0.1.2
	github.com/zmb3/spotify/v2 v2.3.1
	golang.org/x/exp v0.0.0-20230425010034-47ecfdc1ba53
	golang.org/x/oauth2 v0.9.0
	google.golang.org/grpc v1.55.0
	google.golang.org/protobuf v1.30.0
)

require (
	github.com/99designs/gqlgen v0.17.31 // indirect
	github.com/Khan/genqlient v0.6.0 // indirect
	github.com/adrg/xdg v0.4.0 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/charmbracelet/harmonica v0.2.0 // indirect
	github.com/containerd/console v1.0.4-0.20230313162750-1ae8d489ac81 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/iancoleman/strcase v0.2.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-isatty v0.0.18 // indirect
	github.com/mattn/go-localereader v0.0.1 // indirect
	github.com/mattn/go-runewidth v0.0.14 // indirect
	github.com/muesli/ansi v0.0.0-20230316100256-276c6243b2f6 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/muesli/reflow v0.3.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
	github.com/sergi/go-diff v1.3.1 // indirect
	github.com/vektah/gqlparser/v2 v2.5.6 // indirect
	golang.org/x/mod v0.11.0 // indirect
	golang.org/x/net v0.11.0 // indirect
	golang.org/x/sync v0.3.0 // indirect
	golang.org/x/sys v0.9.0 // indirect
	golang.org/x/term v0.9.0 // indirect
	golang.org/x/text v0.10.0 // indirect
	golang.org/x/tools v0.10.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230306155012-7f2fa6fef1f4 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace dagger.io/dagger => github.com/vito/dagger/sdk/go v0.0.0-20230712225502-336f428abfae

replace github.com/dagger/dagger => github.com/vito/dagger v0.0.0-20230712225502-336f428abfae
