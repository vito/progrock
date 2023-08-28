package main

func main() {
	dag.Environment().
		WithCheck(Test).
		WithArtifact(Generate).
		WithArtifact(Demo).
		WithShell(Base).
		Serve()
}

// Test runs tests.
func Test() *EnvironmentCheck {
	return dag.Go().Test(Base(), Code())
}

// Demo builds the demo app.
func Demo() *Directory {
	return dag.Go().Build(Base(), Code(), GoBuildOpts{
		Packages: []string{"./demo"},
		Static:   true,
		Subdir:   "demo",
	})
}

// Generate generates code from .proto files.
func Generate() *Directory {
	return dag.Go().Generate(Base(), Code())
}

func Base() *Container {
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

func Code() *Directory {
	return dag.Host().Directory(".", HostDirectoryOpts{
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
