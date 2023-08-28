package main

import (
	"dagger.io/dagger"
)

var dag = dagger.DefaultClient()

func main() {
	dag.Environment().
		WithCheck_(Test).
		WithArtifact_(Generate).
		WithArtifact_(Demo).
		WithShell_(Base).
		Serve()
}

// Test runs tests.
func Test(ctx dagger.Context) *dagger.EnvironmentCheck {
	return dag.Go().Test(Base(ctx), Code(ctx))
}

// Demo builds the demo app.
func Demo(ctx dagger.Context) *dagger.Directory {
	return dag.Go().Build(
		Base(ctx),
		Code(ctx),
		dagger.GoBuildOpts{
			Packages: []string{"./demo"},
			Static:   true,
			Subdir:   "demo",
		},
	)
}

// Generate generates code from .proto files.
func Generate(ctx dagger.Context) *dagger.Directory {
	return dag.Go().Generate(Base(ctx), Code(ctx))
}

func Base(ctx dagger.Context) *dagger.Container {
	return dag.Apko().Wolfi([]string{
		"go",
		"bash",
		"protobuf-dev", // for google/protobuf/*.proto
		"protoc",
		"protoc-gen-go",
		"protoc-gen-go-grpc",
		"rust",
	}).
		With(dag.Go().BinPath).
		With(dag.Go().GlobalCache).
		WithExec([]string{"go", "install", "gotest.tools/gotestsum@latest"})
}

func Code(ctx dagger.Context) *dagger.Directory {
	return dag.Host().Directory(".", dagger.HostDirectoryOpts{
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
