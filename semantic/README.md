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

## The `gomlx/compute` version

`go.mod` requires `github.com/gomlx/compute` v0.1.16, the first tag that includes
gomlx/compute [#44](https://github.com/gomlx/compute/pull/44) (commit `acb94021`: idle graph
executors release their worker-pool slot, so matmul uses every core; about 3.5x on a
Raspberry Pi 5 over v0.1.15). v0.1.16 also carries #45, a new worker scheduler. Measured on a
Pi 5 over this workspace's database (893 rows, 4,137 passages), the first index took 970 s
and 856 MB at v0.1.16, the same as the 14 to 17 minutes and 875 MB measured on #44 alone.

No `replace` to a fork is committed (workspace DR-0009).
