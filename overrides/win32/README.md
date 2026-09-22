# Win32 overrides

Override JSON is validated against `../schema.json`. An override is rejected if
its recorded `before` fragment no longer matches, which turns upstream fixes or
SDK drift into a reviewable failure instead of silently retaining stale ABI data.
