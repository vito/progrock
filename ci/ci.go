package main

import (
	"dagger.io/dagger"
	"github.com/vito/progrock/ci/pkgs"
)

func main() {
	ctx := dagger.DefaultContext()
	ctx.Client().Environment().
		WithCommand_(Generate).
		WithCheck_(Unit).
		WithCommand_(BuildDemo).
		Serve(ctx)
}

func BuildDemo(ctx dagger.Context) (*dagger.Directory, error) {
	return pkgs.GoBuild(ctx, Base(ctx), Code(ctx), pkgs.GoBuildOpts{
		Packages: []string{"./demo"},
		Static:   true,
		Subdir:   "demo",
	}), nil
}

func Generate(ctx dagger.Context) (*dagger.Directory, error) {
	return pkgs.GoGenerate(ctx, Base(ctx), Code(ctx)), nil
}

func Unit(ctx dagger.Context) (string, error) {
	return pkgs.Gotestsum(ctx, Base(ctx), Code(ctx)).Stdout(ctx)
}

var Wolfi = true

func Base(ctx dagger.Context) *dagger.Container {
	var base *dagger.Container
	if Wolfi {
		base = pkgs.Wolfi(ctx, []string{
			"go",
			"protobuf-dev", // for google/protobuf/*.proto
			"protoc",
			"protoc-gen-go",
			"protoc-gen-go-grpc",
		})
	} else {
		base = pkgs.Nixpkgs(ctx,
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
		With(pkgs.GoBin).
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
