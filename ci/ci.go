package main

import (
	"dagger.io/dagger"
	"github.com/vito/progrock/ci/pkgs"
)

func main() {
	ctx := dagger.DefaultContext()
	ctx.Client().Environment().
		WITHCommand(Generate).
		WITHCommand(Test).
		WITHCommand(BuildDemo).
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

func Test(ctx dagger.Context) (string, error) {
	return pkgs.Gotestsum(ctx, Base(ctx), Code(ctx)).Stdout(ctx)
}

func Base(ctx dagger.Context) *dagger.Container {
	return pkgs.Wolfi(ctx, []string{
		"go",
		"protobuf-dev", // for google/protobuf/*.proto
		"protoc",
		"protoc-gen-go",
		"protoc-gen-go-grpc",
	}).
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
