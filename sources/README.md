# Source inputs

`sources.lock.json` is the source of truth. `winapisource fetch` downloads or
copies each pinned artifact into `sources/cache/<source-id>/`, verifies SHA-256
before extraction, and records no timestamps in generated output. Cache content
is deliberately ignored because Windows SDK/WDK/App SDK redistribution terms
apply to those files.

The committed end-to-end fixture is authored for this project and tests the
pipeline. It is not counted as Microsoft API coverage. Official coverage is
reported only for successfully ingested locked sources.
