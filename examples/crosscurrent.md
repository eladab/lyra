# Crosscurrent

An original miniature for solo piano. **40 bars; 82.6 seconds at the printed
tempi.** The synthesized recording is approximately 87 seconds, including
resonance after the last release. Advanced difficulty.

The opening is a question with a recognisable contour: C–D–G–E–B–A. It acquires
a countercycle, breaks into displaced chord responses, passes between the
hands, and returns in a larger register. The final acceleration is structural:
the pulse stays steady while bars contract from 12 to 9, 6, and 3 eighth notes.
Both hands finally agree on the attacks. A low C and a bright C-Lydian sonority
close the piece with a measured release.

| Time | Bars | Musical development |
| --- | --- | --- |
| 0:00–0:20 | 1–8 | Spark: expose and transform the row; introduce the seven-attack cycle. |
| 0:20–0:33 | 9–16 | Tilt: the same eighth-note pulse becomes 4/4; chord responses anticipate and displace the beat. |
| 0:33–0:53 | 17–24 | Braid: alternating hands become two simultaneous modal fields, then intensify. |
| 0:53–1:03 | 25–28 | Friction: six attacks against four; faster sixteenth-note fragments. |
| 1:03–1:07 | 29–32 | Shear: alternating 7/8 and 5/8 cut the material into shorter cells. |
| 1:07–1:16 | 33–36 | Convergence: the opening returns harmonised, followed by prime and retrograde cascades. |
| 1:16–1:23 | 37–40 | Coda: 9/8 → 6/8 → 3/8, then the final chord and silence. |

The row is **C D G E B A | F♯ C♯ E♭ B♭ F A♭**. Its prime form, tritone
transposition, inversion, retrograde and retrograde inversion organise the
melodic statements. Repeated cells, modal additions and bass pedals are
intentional: this is free serial writing. Bars 19–22 explicitly superpose
**C Lydian** and **F♯ Mixolydian**, exchanging their registers and hands.

The influences shape the composition's rhythmic relationships:

- Ewe Agbadza suggests the relationship between a repeating bell phrase and
  competing beat streams. Here a seven-attack, twelve-pulse cell
  (`x.x.xx.x.x.x`) becomes a recurring anchor. See
  [David Locke's Agbadza teaching and performance](https://dl.tufts.edu/concern/videos/41687z415)
  and his [critical edition](https://sites.tufts.edu/davidlocke/files/2012/01/Agbadza-Critical-Edition-final-DL1.pdf).
- Ghanaian highlife inspires the buoyancy and answering phrases in bars 9–16,
  translated into displaced piano dyads. See
  [UNESCO's account of highlife music and dance](https://ich.unesco.org/en/RL/highlife-music-and-dance-02141).
- Shona mbira music inspires the cyclic, interlocking relationship of the
  hands in bars 17–24. See
  [MBIRA's explanation of leading and intertwining parts](https://mbira.org/what-is-mbira/mbira-music/shona-mbira-music/).

The pitches and melodic phrases are newly composed. These are compositional
influences, rather than arrangements of named traditional pieces.

Play with a clear, dancing pulse. Keep accents different from mere loudness;
release the short notes cleanly. Let the dyads at bars 21–22 sing while the
other hand stays precise. Use no sustaining pedal until the final chord.
Do not add a fermata or slow down before bar 40.

The source includes both print layout and articulated MIDI. The recording uses
FluidSynth with the installed GeneralUser GS acoustic-grand patch; it is a
synthesized performance. Generated PDF, MIDI and audio files live in
`bin/crosscurrent/`. The SoundFont itself is not distributed with the piece.

Open the editable score in Lyra:

```sh
bin/lyra examples/crosscurrent.ly
```

Re-engrave from the project directory:

```sh
lilypond -dno-point-and-click -o bin/crosscurrent/crosscurrent examples/crosscurrent.ly
```
