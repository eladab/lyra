# LilyPond score starters

`templates.json` contains the first complete displayed example from each of the
22 topics in the official [LilyPond 2.26 Learning Manual template appendix](https://lilypond.org/doc/v2.26/Documentation/learning/templates).
These are the current counterparts of the topics in the referenced 2.23 manual.
They are embedded in Lyra, so creating a score needs no network connection.

The excerpts retain the manual's code, including explanatory comments. Lyra adds
a `\version "2.26.0"` line and source/copyright/license comments. The built-in
SATB and SSAATTBB starters use includes supplied by LilyPond itself; no external
project files are required. The quartet-parts example retains its commented
instructions for splitting parts into separate files.

Source: LilyPond Learning Manual, copyright 1998–2026 its authors, under the GNU
Free Documentation License version 1.1 or later, with no Invariant Sections.
The license is included in [COPYING.FDL](COPYING.FDL). Source URLs are retained per
template and in each generated score. Search aliases and blank-file handling are
Lyra additions.

To reproduce the catalog from the saved official `learning-big-page.html`:

```sh
python3 scripts/import-lilypond-templates.py /path/to/learning-big-page.html
```

The importer checks the topic inventory and extracts plain code, excluding
lilypond-book's generated rendering wrapper. `LYRA_TEST_LILYPOND=/path/to/lilypond
go test ./internal/scoretemplate` engraves every starter in private test folders.
