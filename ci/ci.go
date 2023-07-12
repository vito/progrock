package main

import (
	"dagger.io/dagger"
	"github.com/dagger/dagger/universe/apkoenv"
	"github.com/dagger/dagger/universe/goenv"
	"github.com/dagger/dagger/universe/nixenv"
)

func main() {
	ctx := dagger.DefaultContext()
	ctx.Client().Environment().
		WithCheck_(Unit).
		WithCommand_(Generate).
		WithCommand_(BuildDemo).
		Serve(ctx)
}

func BuildDemo(ctx dagger.Context) (*dagger.Directory, error) {
	return goenv.Build(ctx, Base(ctx), Code(ctx), goenv.GoBuildOpts{
		Packages: []string{"./demo"},
		Static:   true,
		Subdir:   "demo",
	}), nil
}

func Generate(ctx dagger.Context) (*dagger.Directory, error) {
	return goenv.Generate(ctx, Base(ctx), Code(ctx)), nil
}

func Unit(ctx dagger.Context) (string, error) {
	return goenv.Gotestsum(ctx, Base(ctx), Code(ctx)).Stdout(ctx)
}

var Wolfi = true

func Base(ctx dagger.Context) *dagger.Container {
	var base *dagger.Container
	if Wolfi {
		base = apkoenv.Wolfi(ctx, []string{
			"go",
			"protobuf-dev", // for google/protobuf/*.proto
			"protoc",
			"protoc-gen-go",
			"protoc-gen-go-grpc",
		})
	} else {
		base = nixenv.Nixpkgs(ctx,
			ctx.Client(). // TODO: it'd be great to memoize this
					Git("https://github.com/nixos/nixpkgs").
					Branch("nixos-unstable").
					Tree(),
			"go",
			"protobuf", // for google/protobuf/*.proto
			"protoc-gen-go",
			"protoc-gen-go-grpc",
		)
	}

	return base.
		With(goenv.BinPath).
		WithExec([]string{"go", "install", "gotest.tools/gotestsum@latest"})
}

func Code(ctx dagger.Context) *dagger.Directory {
	return ctx.Client().Host().Directory(".", dagger.HostDirectoryOpts{
		Include: []string{
			"**/*.go",
			"**/go.mod",
			"**/go.sum",
			"**/testdata/**/*",
			"**/*.proto",
			"**/*.tmpl",
		},
		Exclude: []string{
			"ci/**/*",
		},
	})
}
