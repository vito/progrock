package main

import (
	"io"
	"os"
	"strings"

	"github.com/vito/dagql/idproto"
	"github.com/vito/progrock"
	"github.com/vito/progrock/ui"
)

func main() {
	bytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		panic(err)
	}
	var idp idproto.ID
	if err := idp.Decode(strings.TrimSpace(string(bytes))); err != nil {
		panic(err)
	}
	out := ui.NewOutput(os.Stdout)
	if err := progrock.DebugRenderID(out, progrock.DefaultUI(), &idp, 0); err != nil {
		panic(err)
	}
}
