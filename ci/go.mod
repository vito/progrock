module ci

go 1.20

replace dagger.io/dagger => github.com/vito/dagger/sdk/go v0.0.0-20230827220824-19fe2de0c22a

require (
	dagger.io/dagger v0.0.0-00010101000000-000000000000
	github.com/Khan/genqlient v0.6.0
	github.com/iancoleman/strcase v0.3.0
	github.com/vektah/gqlparser/v2 v2.5.6
)

require (
	github.com/99designs/gqlgen v0.17.31 // indirect
	github.com/adrg/xdg v0.4.0 // indirect
	golang.org/x/mod v0.12.0 // indirect
	golang.org/x/sync v0.3.0 // indirect
	golang.org/x/sys v0.10.0 // indirect
	golang.org/x/tools v0.11.0 // indirect
)
