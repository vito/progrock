package main

import "dagger.io/dagger"

func main() {
	dagger.ServeCommands(
		Generate,
		Test,
	)
}

func Generate(ctx dagger.Context) (*dagger.Directory, error) {
	c := ctx.Client()
	return Biome(ctx).
		WithMountedDirectory("/src", c.Host().Directory(".")).
		WithWorkdir("/src").
		WithExec([]string{"go", "generate", "./..."}).
		Directory("/src"), nil
}

func Test(ctx dagger.Context) (string, error) {
	c := ctx.Client()
	return Biome(ctx).
		WithMountedDirectory("/src", c.Host().Directory(".")).
		WithWorkdir("/src").
		// TODO should mark this 'focused' somehow
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
	return NixImage(ctx, Flake(ctx),
		"bashInteractive",
		"go_1_20",
		"protobuf",
		"protoc-gen-go",
		"protoc-gen-go-grpc",
		"gotestsum",
	)
}

func Flake(ctx dagger.Context) *dagger.Directory {
	return ctx.Client().Host().Directory(".", dagger.HostDirectoryOpts{
		// NB: maintain this as-needed, in case the Nix code sprawls
		Include: []string{"flake.nix", "flake.lock"},
	})
}
