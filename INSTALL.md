Installation **knowledge**
============================

**knowledge** A standalone SQLite3-backed knowledge base for tracking projects, observations, and concepts across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (Open, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool, so other experiments and language-model harnesses can read and write structured observations directly instead of raw sqlite3 CLI inserts. Ships cmd/kb, a single "kb VERB ARGS" binary (matching the git/go command model) covering the full API with both human-readable and --json output, a merge verb for reconciling two databases that drifted independently, and a read-only interactive TUI browser for exploring projects, observations, and concepts.

Quick install with curl or irm
------------------------------

There is an experimental installer.sh script that can be run with the following command to install latest table release. This may work for macOS, Linux and if you’re using Windows with the Unix subsystem. This would be run from your shell (e.g. Terminal on macOS).

~~~shell
curl https://rsdoiel.github.io/knowledge/installer.sh | sh
~~~

This will install the programs included in knowledge in your `$HOME/bin` directory.

If you are running Windows 10 or 11 use the Powershell command below.

~~~ps1
irm https://rsdoiel.github.io/knowledge/installer.ps1 | iex
~~~

### If your are running macOS or Windows

You may get security warnings if you are using macOS or Windows. See the notes for the specific operating system you’re using to fix issues.

- [INSTALL_NOTES_macOS.md](INSTALL_NOTES_macOS.md)
- [INSTALL_NOTES_Windows.md](INSTALL_NOTES_Windows.md)

Installing from source
----------------------

### Required software

- Go >= 1.27.3

### Steps

1. git clone https://github.com/rsdoiel/knowledge
2. Change directory into the `knowledge` directory
3. Make to build, test and install

~~~shell
git clone https://github.com/rsdoiel/knowledge
cd knowledge
make
make test
make install
~~~

Semantic search (preview)
-------------------------

Semantic search (`kb search -semantic`, `-hybrid`) needs a separate build of `kb`: the plain one registers no embedder. The semantic build is not yet in the release archives; build it from source.

### Required software

- Go >= 1.27.3 (the embedding library needs it, and so does the plain `kb`)
- No C compiler, and no shared libraries: the build is pure Go (`CGO_ENABLED=0`)

### Steps

~~~shell
git clone https://github.com/rsdoiel/knowledge
cd knowledge
make semantic            # builds bin/kb-semantic through go.work
~~~

`make test-semantic` runs the nested module's tests. `go.work` joins the nested module to the working tree, so `go` run inside `semantic/` sees your changes to the root module.

Put `bin/kb-semantic` on your path as `kb` (or beside it). Then, in a workspace:

~~~shell
kb init                  # keeps a workspace model and the index out of git
kb model fetch           # about 87 MB from Hugging Face, checked against recorded SHA-256 sums
kb embed                 # the first run: 14 to 17 minutes on a Raspberry Pi 5 for 893 rows
~~~

With no network, copy the model from another machine and run `kb model install PATH` (a directory, or a .zip or .tar.gz holding it). By default the model is installed for your user (`$XDG_DATA_HOME/kb/models`, or `~/.local/share/kb/models`); add `-workspace` to `kb model fetch` or `install` to keep it in the workspace's `agents/models/` instead. The first index used up to 875 MB of memory on the Pi; the binary is about 33 MB. See [kb-model(1)](kb-model.1.md) and [kb-embed(1)](kb-embed.1.md).
