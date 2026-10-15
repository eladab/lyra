.DEFAULT_GOAL := build

GO ?= go
BREW ?= brew
BINDIR ?= $(HOME)/.local/bin
DATADIR ?= $(if $(XDG_DATA_HOME),$(XDG_DATA_HOME),$(HOME)/.local/share)/lyra

.PHONY: build install deps soundfont test check

build:
	"$(GO)" build -o bin/lyra ./cmd/lyra

deps:
	@command -v "$(BREW)" >/dev/null 2>&1 || { printf '%s\n' 'Homebrew is required. Install it from https://brew.sh, then rerun make install.' >&2; exit 1; }
	"$(BREW)" bundle install --no-upgrade --file=Brewfile

# Build after deps, including with make -j: Go may be installed by that target.
install: deps soundfont
	$(MAKE) build
	install -d "$(BINDIR)"
	install -m 0755 bin/lyra "$(BINDIR)/lyra"
	@printf 'Installed Lyra to %s/lyra\n' "$(BINDIR)"

soundfont:
	sh scripts/install-soundfont.sh "$(DATADIR)/soundfonts"

test:
	"$(GO)" test -race ./...

check:
	"$(GO)" vet ./...
	"$(GO)" test -race ./...
