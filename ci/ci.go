package main

import (
	"dagger.io/dagger"
)

func main() {
	dagger.ServeCommands(
		Generate,
		Test,
	)
}

func Generate(ctx dagger.Context) (*dagger.Directory, error) {
	return Biome(ctx).
		Focus().
		WithExec([]string{"go", "generate", "./..."}).
		Directory("/src"), nil
}

func Test(ctx dagger.Context) (string, error) {
	return Biome(ctx).
		Focus().
		WithExec([]string{
			"gotestsum",
			"--format=testname",
			"--no-color=false",
			"./...",
		}).
		// TODO would prefer to just call .Sync here, or nothing at all.
		Stdout(ctx)
}

func Biome(ctx dagger.Context) *dagger.Container {
	return Nixpkgs(ctx, Flake(ctx),
		"bashInteractive",
		"go_1_20",
		"protobuf",
		"protoc-gen-go",
		"protoc-gen-go-grpc",
		"gotestsum",
	).
		WithEnvVariable("GOCACHE", "/go/build-cache").
		WithMountedCache("/go/pkg/mod", ctx.Client().CacheVolume("go-mod")).
		WithMountedCache("/go/build-cache", ctx.Client().CacheVolume("go-build")).
		WithMountedDirectory("/src", Code(ctx)).
		WithWorkdir("/src")
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
	})
}

func Flake(ctx dagger.Context) *dagger.Directory {
	return ctx.Client().Host().Directory(".", dagger.HostDirectoryOpts{
		// NB: maintain this as-needed, in case the Nix code sprawls
		Include: []string{"flake.nix", "flake.lock"},
	})
}
