# Tikvar: closing excerpt for solo piano

Source: `/Users/elad/Downloads/tikvar.mp3`, from **3:04.000 to the end at approximately 3:54.475**. The source filename supplies the working title; the composer and original title have not been identified.

Open the editable score with:

```sh
lyra /Users/elad/dev/lyra/scores/tikvar-piano.ly
```

The score contains both engraving and acoustic-grand-piano MIDI output. The checked renderings are in `/Users/elad/dev/lyra/bin/tikvar-piano/`: PDF, MIDI, WAV and MP3. To regenerate PDF and MIDI from the project directory:

```sh
lilypond -o bin/tikvar-piano/tikvar-piano scores/tikvar-piano.ly
```

## What this transcription represents

This is a **computer-assisted, best-effort transcription and two-hand piano reduction**, not an aurally verified critical edition. Pitches were estimated using the [high-resolution piano transcription model](https://github.com/qiuqiangkong/piano_transcription_inference), cross-checked with [Spotify Basic Pitch](https://github.com/spotify/basic-pitch), and checked against spectral peaks in the supplied excerpt. Several missing upper notes in the first estimate were recovered during that cross-check.

The slow pulse is notated at approximately quarter note = 50. Barlines, the 4/4 grouping, the final 5/4 gesture, dynamics, pedaling advice and note spellings are editorial. The opening rest represents about 0.46 seconds before the first attack. The last bass attack occurs at about 3:45.33; its decay and the recording's trailing silence account for the remaining duration.

The detected principal melody and bass registers are retained. Inner voices in the dense middle passage, especially measures 5-7, are simplified or moved by octaves to keep each hand manageable. The two wider intervals in measure 4 are marked to roll. The source's rolled attacks are compressed into notated chords; MIDI does not reproduce every stagger within those rolls. Long detector-generated overlaps were regularized to readable note values.

Pitch uncertainty is greatest around **3:20, 3:33 and 3:37**, where the detectors disagree on F/F-sharp or E/E-flat components. The selected readings follow the stronger spectral evidence, but these locations merit an ear check. Chord voicings in the dense passage are an arrangement, not a claim that every notated octave was present in the recording. No conventional key signature is asserted.

## Verification

- LilyPond 2.26.0 compiles the score to a one-page A4 PDF and two piano MIDI tracks without warnings.
- The rendered PDF was visually checked for clipping, collisions and legibility.
- The MIDI has 33 grouped attacks, a range of F2-F-sharp6, and a final note ending at 47.29 seconds from the excerpt start. Its grouped attack times are within 0.05 seconds of the selected source attack grid. This checks timing, not pitch accuracy.
- FluidSynth rendered the MIDI successfully with the installed GeneralUser GS SoundFont. The MP3 preview is approximately 50.15 seconds including the final decay.

The original MP3, existing compositions, application code and user configuration were not modified. Analysis files and isolated Python dependencies are under the ignored `work/tikvar-transcription/` directory; generated audio and engraving are under the ignored `bin/tikvar-piano/` directory.
