package main

import (
	"bytes"
	_ "embed"
	"strings"
	"text/template"

	"dagger.io/dagger"
)

func NixImage(ctx dagger.Context, flake *dagger.Directory, packages ...string) *dagger.Container {
	// NB: it's tempting to do this, but I've seen cases where order matters.
	// sort.Strings(packages)

	imageRef := "nixpkgs/" + strings.Join(packages, "/")
	drv := nixDerivation(ctx, "/flake", imageRef, packages...)

	result := nixResult(ctx,
		nixBase(ctx).
			WithMountedDirectory("/src", drv).
			WithMountedDirectory("/flake", flake).
			// TODO: --option filter-syscalls false to let Apple Silicon
			// cross-compile to Intel
			WithExec([]string{"nix", "build", "-f", "/src/image.nix"}))

	return ctx.Client().Container().
		Import(result).
		WithMountedTemp("/tmp")
}

func NixImageLayout(ctx dagger.Context, flake *dagger.Directory, packages ...string) *dagger.Container {
	imageRef := "nixpkgs/" + strings.Join(packages, "/")
	drv := nixDerivation(ctx, "/flake", imageRef, packages...)

	build :=
		nixBase(ctx).
			WithExec([]string{"nix", "profile", "install", "nixpkgs#skopeo"}).
			WithMountedDirectory("/src", drv).
			WithMountedDirectory("/flake", flake).
			WithMountedTemp("/tmp").
			// TODO: --option filter-syscalls false to let Apple Silicon
			// cross-compile to Intel
			WithExec([]string{"nix", "build", "-f", "/src/image.nix"}, dagger.ContainerWithExecOpts{
				Focus: true,
			}).
			WithExec([]string{
				"skopeo", "--insecure-policy",
				"copy", "docker-archive:./result", "oci:./layout:latest",
			}, dagger.ContainerWithExecOpts{
				Focus: true,
			})

	return ctx.Client().Container().
		ImportDir(build.Directory("./layout")).
		WithMountedTemp("/tmp")
}

func nixBase(ctx dagger.Context) *dagger.Container {
	c := ctx.Client()

	base := c.Container().
		From("nixos/nix")

	return base.
		With(nixCache(c)).
		WithExec([]string{"sh", "-c", "echo accept-flake-config = true >> /etc/nix/nix.conf"}).
		WithExec([]string{"sh", "-c", "echo experimental-features = nix-command flakes >> /etc/nix/nix.conf"})
}

func nixCache(c *dagger.Client) dagger.WithContainerFunc {
	return func(ctr *dagger.Container) *dagger.Container {
		return ctr.WithMountedCache(
			"/nix/store",
			c.CacheVolume("nix-store"),
			dagger.ContainerWithMountedCacheOpts{
				Source: c.Container().From("nixos/nix").Directory("/nix/store"),
			})
	}
}

func nixResult(ctx dagger.Context, ctr *dagger.Container) *dagger.File {
	return ctr.
		WithExec([]string{"cp", "-aL", "./result", "./exported"}).
		File("./exported")
}

//go:embed image.nix.tmpl
var imageNixSrc string

var imageNixTmpl *template.Template

func init() {
	imageNixTmpl = template.Must(template.New("image.nix.tmpl").Parse(imageNixSrc))
}

func nixDerivation(ctx dagger.Context, flakeRef, name string, packages ...string) *dagger.Directory {
	w := new(bytes.Buffer)
	err := imageNixTmpl.Execute(w, struct {
		FlakeRef string
		Name     string
		Packages []string
	}{
		FlakeRef: flakeRef,
		Name:     name,
		Packages: packages,
	})
	if err != nil {
		panic(err)
	}

	return ctx.Client().Directory().WithNewFile("image.nix", w.String())
}
