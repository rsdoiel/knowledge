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
gomlx/compute [#44](https://github.com/gomlx/compute/pull/44) (merged 2026-10-28: idle graph
executors release their worker-pool slot, so matmul uses every core; about 3.5x on a
Raspberry Pi 5 over v0.1.15). v0.1.16 also carries #45, a new worker scheduler.

## The `hugot` version

`go.mod` requires `github.com/knights-analytics/hugot` v0.8.5 or later. Before v0.8.5 the
pure-Go backend compiled and kept an executable of about 130 MB for every distinct input
shape, so length-sorted batches took a first index past 14 GB; the embedder worked round it
by replacing the session every four shapes. v0.8.5 compiles with dynamic axes instead
([hugot#154](https://github.com/knights-analytics/hugot/issues/154)), so one session serves
every shape and the workaround is gone.

Measured on a Pi 5 over a copy of this workspace's database (947 rows, 4,380 passages), the
first index took 15 min 36 s and peaked at 1,190 MB with hugot v0.8.5 and one session. The
same data with hugot v0.8.3 and the replace-every-four-shapes workaround took 19 min 46 s and
peaked at 868 MB (the Pi's soft temperature limit was active at the end of that run, so the
speed gap is probably overstated). Memory is bounded: the compiled shapes are kept, at
a few MB each, and there is no per-shape cap.

No `replace` to a fork is committed (workspace DR-0009).
