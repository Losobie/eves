# Read-only blue.Marshal decoder

Adapted from the reader and wire definitions in
[TrueBrain/blue-marshal-rs](https://github.com/TrueBrain/blue-marshal-rs/tree/fa99764271f5a70a9565ab6e6f092f57617382a8)
(MIT; see LICENSE, including CCP Games' notice).

This decoder reads versions 0 and 1, shared references, string-table entries,
numeric/string/container types, object wrappers, and Adler-32 checksums. Object
wrappers are represented as data; no deserialization callbacks are executed.
It is not an encoder or a lossless editing API.

`ToJSON` exports the complete decoded value using the upstream typed JSON
convention from `src/json.rs`. It preserves typed dictionary keys, tuples,
byte/Unicode strings, large integers, non-finite floats, and object wrappers.
It expands shared references, with independent nesting, node, and estimated
size limits. Duplicate JSON keys and unsupported object payload shapes error
instead of silently dropping data. No JSON-to-marshal encoder is provided.

As in the reference library, database rows, nested streams, pickle payloads, and
complex numbers are unsupported. Unknown tags, cyclic/unresolved references,
invalid encodings, bad checksums, and trailing data cause errors. Limits are
64 MiB input, 256 nested levels, and one million decoded objects.

The binary fixtures in testdata were generated using the independently
published @truebrain/blue-marshal 1.0.1 WASM encoder and verified using its
decoder. Their matching typed JSON is included for inspection. They contain
synthetic settings only, including Unicode, user/internal formations, large
timestamps, and unrelated settings. `testdata/generate.mjs` documents how to
regenerate them. Node and the WASM package are only needed for regeneration,
not building, testing, or running eves.
