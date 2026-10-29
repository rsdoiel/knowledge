%kb-model(1) user manual | version 0.0.29 7d292d7
% R. S. Doiel
% 2026-10-29

# NAME

kb-model — install, fetch and list the embedding models semantic search uses

# SYNOPSIS

kb model install PATH [-name NAME] [-workspace] [-dir DIR]

kb model fetch [NAME] [-source URL] [-workspace] [-dir DIR]

kb model list [-workspace] [-dir DIR]

# DESCRIPTION

Semantic search compares the meaning of a question with the meaning of the
rows in a knowledge base, using an embedding model: a few files, about 87 MB for
the default, all-MiniLM-L6-v2 (Apache-2.0). model puts those files where
kb looks for them. It only manages files. It opens no knowledge base,
and it works the same in a kb built without semantic search, which can
install a model but not use it; list says which kind of build is running.

install PATH
: copies a model from a directory, or from a .zip, .tar.gz or .tgz archive
  holding one, after checking each file against the size and SHA-256 kb
  has on record for the model. It uses no network. Without PATH it exits 2 and
  points to fetch and to install PATH. The model is recognised from its files;
  -name names it explicitly.

fetch [NAME]
: downloads the model (the default when NAME is omitted), checks each file as it
  arrives, and installs it. This is the only place kb uses the network,
  and only when you run it. -source URL fetches the files from a mirror (the
  files at URL/FILE) instead of the recorded address.

list
: shows the build's capability, each installed model with where it is and
  whether its files are all present, and the model the vector index was built
  with. -json prints the same as a document.

Nothing is left half-installed: files are copied or downloaded into a scratch
directory that is moved into place only when every file checks out. An
existing model of the same name is never overwritten; remove its directory
first to replace it.

# OPTIONS

-workspace
: install into WORKSPACE/agents/models/ instead of the user directory. The
  workspace is the one found at or above the current directory. In a
  git work tree kb adds agents/models/ to the workspace's .gitignore
  if it is missing (the files are large and are not part of the knowledge), and
  warns if git already tracks them.

-dir DIR
: install into, and look in, DIR instead (KB_MODEL_DIR does the same).

-name NAME
: install: the model the files are, when they cannot be recognised.

-source URL
: fetch: the base address to download from.

The global -json option applies. The global -db option does not: model opens no
database, and an explicit -db is refused (exit 2).

# FILES

Models are looked for in WORKSPACE/agents/models/NAME/ first, then in the user
data directory ($XDG_DATA_HOME/kb/models, or ~/.local/share/kb/models;
~/Library/Application Support/kb/models on macOS; %LocalAppData%\kb\models on
Windows). KB_MODEL_DIR, or -dir, replaces both. Each installed model has a
kb-model.json recording what was installed.

# EXAMPLES

~~~shell
kb model fetch
kb model install ~/Downloads/all-MiniLM-L6-v2.zip
kb model install -workspace /media/usb/all-MiniLM-L6-v2
kb model list
~~~

# EXIT STATUS

The workspace convention, as described in kb(1).

0
: the model was installed, or list ran

2
: a bad flag, a missing PATH, a surplus argument, an unknown subverb or model name, or a -db

65
: the files are not the model: one is missing, the wrong size, or fails its checksum; or an
  archive is unreadable or holds a path outside itself

66
: PATH does not exist, a download was refused as not found, or -workspace found no workspace

69
: fetch could not reach the source, or it answered with an error

73
: a model of that name is already installed, or the directory cannot be created

74
: a copy or a download stopped part way

77
: the operating system refused access to a directory or file

# SEE ALSO

kb(1), kb-init(1), kb-search(1)
