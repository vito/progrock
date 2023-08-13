package main

import (
	"dagger.io/dagger"
)

var dag = dagger.DefaultClient()

func main() {
	dag.Environment().
		WithCheck_(Test).
		WithArtifact_(Generate).
		WithArtifact_(BuildDemo).
		Serve()
}

func BuildDemo(ctx dagger.Context) *dagger.Directory {
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

func Generate(ctx dagger.Context) *dagger.Directory {
	return dag.Go().Generate(Base(ctx), Code(ctx))
}

func Test(ctx dagger.Context) *dagger.EnvironmentCheck {
	return dag.Go().Test(Base(ctx), Code(ctx))
}

func Base(ctx dagger.Context) *dagger.Container {
	return dag.Apko().Wolfi([]string{
		"go",
		"protobuf-dev", // for google/protobuf/*.proto
		"protoc",
		"protoc-gen-go",
		"protoc-gen-go-grpc",
		"rust",
	}).
		WithEnvVariable("GOBIN", "/go/bin").
		WithEnvVariable("PATH", "$GOBIN:$PATH", dagger.ContainerWithEnvVariableOpts{
			Expand: true,
		}).
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
