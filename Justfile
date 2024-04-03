set export

go := "go"
gofmt := "gofmt"

tool_govulncheck := "golang.org/x/vuln/cmd/govulncheck@latest"
tool_deadcode := "golang.org/x/tools/cmd/deadcode@latest"
tool_staticcheck := "honnef.co/go/tools/cmd/staticcheck@latest"
tool_templ := "github.com/a-h/templ/cmd/templ@latest"

build: gen
	{{go}} install -v -race ./...

watch-build:
	watchexec --print-events --debounce 1s -e go {{go}} install -v -race ./...

gen-template:
	@printf "\x1b[34m===>\x1b[m  Running templ generate\n"
	@{{go}} run {{tool_templ}} generate

gen-go:
	@printf "\x1b[34m===>\x1b[m  Running go generate\n"
	@{{go}} generate ./...

gen: gen-template gen-go

check:
	#!/usr/bin/env fish

	printf "\x1b[34m===>\x1b[m  Running go vet check\n"
	{{go}} vet ./...

	printf "\x1b[34m===>\x1b[m  Running govulncheck\n"
	{{go}} run {{tool_govulncheck}} ./... || exit 1

	printf "\x1b[34m===>\x1b[m  Running deadcode\n"
	{{go}} run {{tool_deadcode}} -test ./... || exit 1

	printf "\x1b[34m===>\x1b[m  Running staticcheck\n"
	{{go}} run {{tool_staticcheck}} -go 1.22 ./... || exit 1

	printf "\x1b[34m===>\x1b[m  Running gofmt check\n"
	set -l _var (gofmt -s -l .)
	if test -n "$_var"
		printf "\x1b[31m===>\x1b[m  some files are not formatted properly\n"
		printf "$_var\n"
		exit 1
	end

fmt:
	#!/usr/bin/env fish

	printf "\x1b[34m===>\x1b[m  Running gofmt\n"
	{{gofmt}} -s -w . || exit 1

	printf "\x1b[34m===>\x1b[m  Running templ fmt\n"
	{{go}} run {{tool_templ}} fmt . || exit 1
