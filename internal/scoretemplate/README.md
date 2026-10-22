# LilyPond score starters

`templates.json` contains the first complete displayed example from each of the
22 topics in the official [LilyPond 2.26 Learning Manual template appendix](https://lilypond.org/doc/v2.26/Documentation/learning/templates).
These are the current counterparts of the topics in the referenced 2.23 manual.
They are embedded alongside 15 original Lyra ensemble starters, so creating a
score needs no network connection. The picker has 38 choices including blank.

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

To reproduce the manual catalog from the saved official `learning-big-page.html`:

```sh
python3 scripts/import-lilypond-templates.py /path/to/learning-big-page.html
```

The importer checks the topic inventory and extracts plain code, excluding
lilypond-book's generated rendering wrapper. It does not touch the Lyra additions.

## Lyra ensemble starters

`ensembles.json` holds searchable names and instrument aliases; `ensembles/*.ly`
holds the editable, self-contained LilyPond sources. These are original Lyra
starters, not additional excerpts attributed to the manual. The existing official
string quartet and string quartet parts templates remain available.

| Starter | Instrumentation, in score order |
| --- | --- |
| String trio | Violin, viola, cello |
| String quintet (two violas) | 2 violins, 2 violas, cello |
| String quintet (two cellos) | 2 violins, viola, 2 cellos |
| String sextet | 2 violins, 2 violas, 2 cellos |
| Piano trio | Violin, cello, piano |
| Piano quartet | Violin, viola, cello, piano |
| Piano quintet | 2 violins, viola, cello, piano |
| Clarinet quintet | B-flat clarinet, 2 violins, viola, cello |
| Woodwind quintet | Flute, oboe, B-flat clarinet, F horn, bassoon |
| Brass quintet | 2 B-flat trumpets, F horn, trombone, tuba |
| String orchestra | Violins I/II, violas, cellos, double basses |
| Chamber orchestra (single winds) | Flute, oboe, B-flat clarinet, bassoon; 2 F horns; strings |
| Chamber orchestra (Classical) | 2 flutes, 2 oboes, 2 B-flat clarinets, 2 bassoons; 2 F horns, 2 B-flat trumpets; timpani; strings |
| Symphony orchestra (double winds) | 2 flutes, 2 oboes, 2 B-flat clarinets, 2 bassoons; 4 F horns, 2 B-flat trumpets, 2 tenor trombones, bass trombone, tuba; timpani; strings |
| Symphony orchestra (expanded winds) | Piccolo, 2 flutes, 2 oboes, English horn, 2 B-flat clarinets, B-flat bass clarinet, 2 bassoons, contrabassoon; 4 F horns, 3 B-flat trumpets, 2 tenor trombones, bass trombone, tuba; timpani, snare drum; harp; strings |

Orchestral forces are explicit choices, not a claim that there is one universal
chamber or symphony orchestra. Strings have five independent section staves;
player counts are left to the composer. Winds and brass have individual player
staves. The expanded orchestra uses dedicated auxiliary woodwind players, without
implicit doublings, and a specifically labeled snare-drum staff. Add or change
percussion instruments as required by the piece.

Each starter has four bars of full-measure rests, shared key/meter/tempo, named
music variables, full and abbreviated instrument labels, conventional score
order and clefs, brackets for families, and braces for piano/harp. Staves stay
visible while empty. Chamber scores use A4; the three larger orchestras use A3
with smaller staves. These settings are editable in `\paper` and the staff-size
declaration. Both `\layout` and `\midi` are enabled. Playback is silent until
the composer replaces rests with notes.

All pitched `*Music` variables use **concert pitch**. The score applies
[`\transpose` and `\transposition`](https://lilypond.org/doc/v2.26/Documentation/notation/displaying-pitches#instrument-transpositions)
to print written pitches while preserving sounding MIDI pitches: B-flat clarinet
and trumpet sound a major second lower, F horn and English horn a fifth lower,
treble-clef bass clarinet a major ninth lower, double bass and contrabassoon an
octave lower, and piccolo an octave higher. Clefs are ordinary bass/treble/alto;
octave-transposing instruments do not get a second octave shift from the clef.
The scores use modern key signatures, including horns. The clarinet quintet uses
B-flat clarinet and includes instructions for changing it to A clarinet.

MIDI uses [one channel per sound](https://lilypond.org/doc/v2.26/Documentation/notation/midi-channel-mapping)
to avoid channel exhaustion in large scores. Orchestra strings share the General
MIDI string-ensemble sound; bass clarinet and contrabassoon use clarinet and bassoon
sounds because General MIDI has no dedicated patches for them. The expanded
orchestra uses 13 pitched sounds plus the percussion channel. Staves sharing a
sound also share controllers such as MIDI volume and pan.

## Verification

```sh
LYRA_TEST_LILYPOND=/path/to/lilypond go test ./internal/scoretemplate
```

This engraves all 37 starters in private test folders, requires warning-free PDF
and MIDI generation for the Lyra additions, and injects distinct test notes to
check every new staff's sounding MIDI pitch and instrument (including snare on
channel 10). Ordinary tests check instrumentation, score order, attribution and
search; launcher tests cover creation from both catalogs. When editing sources,
also render the PDFs with Poppler and inspect grouping, labels, clefs, key
signatures, spacing and clipping, including a score with multiple systems.
