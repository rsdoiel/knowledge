Installation **knowledge**
============================

**knowledge** A standalone SQLite3-backed knowledge base for tracking projects, observations, and concepts across independent experiments in the rsdoiel/Laboratory workspace. Extracted from the harvey terminal agent's knowledge.go, this module provides a typed CRUD API (OpenKnowledgeBase, AddProject, AddObservationWithSource, AddConceptWithIdentifier, Search, and related methods) plus UUID-based row identity and a SQL/ATTACH-based cross-machine merge tool (cmd/kbmerge), so other experiments can write structured observations directly through a typed API instead of raw sqlite3 CLI inserts, and harvey becomes a consumer of this module rather than its owner.

Quick install with curl or irm
------------------------------

There is an experimental installer.sh script that can be run with the following command to install latest table release. This may work for macOS, Linux and if you’re using Windows with the Unix subsystem. This would be run from your shell (e.g. Terminal on macOS).

~~~shell
curl https://Laboratory.github.io/knowledge/installer.sh | sh
~~~

This will install the programs included in knowledge in your `$HOME/bin` directory.

If you are running Windows 10 or 11 use the Powershell command below.

~~~ps1
irm https://Laboratory.github.io/knowledge/installer.ps1 | iex
~~~

### If your are running macOS or Windows

You may get security warnings if you are using macOS or Windows. See the notes for the specific operating system you’re using to fix issues.

- [INSTALL_NOTES_macOS.md](INSTALL_NOTES_macOS.md)
- [INSTALL_NOTES_Windows.md](INSTALL_NOTES_Windows.md)

Installing from source
----------------------

### Required software

- Go >= 1.26.3

### Steps

1. git clone https://github.com/Laboratory/knowledge
2. Change directory into the `knowledge` directory
3. Make to build, test and install

~~~shell
git clone https://github.com/Laboratory/knowledge
cd knowledge
make
make test
make install
~~~

