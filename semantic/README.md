# semantic

The Hugot embedder for `kb` semantic search (DR-0088, DR-0089). It is a nested module so
the Hugot and GoMLX dependency tree reaches only the program built from here
(`cmd/kb`), never a program that imports `github.com/rsdoiel/knowledge`.

`NewHugot(dir)` loads all-MiniLM-L6-v2 from a model directory on first use, runs it in
length-sorted batches of 8, returns unit-length 384-dimension vectors, and cuts any input
over 512 tokens to fit (one longer input otherwise crashes the whole batch). `Fingerprint`
is the model name plus a hash of `model.onnx`, `tokenizer.json` and `config.json`.

## Tests

Tests that need the model are skipped when it is absent. Point `KB_TEST_MODEL_DIR` at a
directory holding the model, or use the spike's copy under `tmp/hugot-spike/models/`.

## The `gomlx/compute` pin

`go.mod` pins `github.com/gomlx/compute` to the pseudo-version
`v0.1.16-0.20261028143122-acb940214a36` (upstream commit `acb94021`). That commit is
gomlx/compute [#44](https://github.com/gomlx/compute/pull/44): idle graph executors release
their worker-pool slot, so matmul uses every core. It speeds indexing about 3.5x on a
Raspberry Pi 5. The latest tag, v0.1.15, predates it.

No `replace` to a fork is committed (workspace DR-0009). **Bump to the first tagged release
that includes `acb94021`** (`go get github.com/gomlx/compute@vX.Y.Z`), and do not cut a `kb`
release on the pseudo-version.
