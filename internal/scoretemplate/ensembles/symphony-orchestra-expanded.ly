% Orchestral - Symphony orchestra (expanded winds)
% Source: Lyra ensemble library
% Original Lyra starter, not an excerpt from the LilyPond manual.
% Instrumentation: Picc, 2 fl; 2 ob, E hn; 2 cl, B cl; 2 bsn, Cbsn.
% Enter all pitched music at CONCERT pitch in the *Music variables below.
% The score converts it to written pitch and declares each MIDI transposition.
% Reference: https://lilypond.org/doc/v2.26/Documentation/notation/displaying-pitches#instrument-transpositions
% Replace each R1*4 with your music; keep all parts the same total duration.
% Change the shared key, meter and tempo in global. R1*4 is four bars in 4/4.
% Brass: 4 horns, 3 trumpets, 2 tenor trombones, bass trombone, tuba.
% Also timpani, one snare-drum player, harp and five string sections.
% Auxiliary woodwinds have dedicated players (no hidden doublings).
% Bass clarinet uses modern treble-clef notation, sounding a major ninth lower.
% The percussion staff is specifically snare drum; adapt it for other instruments.
% String staves represent sections; choose the number of players separately.
% Section strings share a General MIDI ensemble sound.
% Bass clarinet and contrabassoon use General MIDI clarinet and bassoon sounds.
\version "2.26.0"

#(set-global-staff-size 11)

\header {
  title = "Untitled"
  subtitle = "Symphony orchestra (expanded winds)"
  composer = "Composer"
  tagline = ##f
}

\paper {
  #(set-paper-size "a3")
  indent = 38\mm
  short-indent = 18\mm
  ragged-last = ##f
  ragged-bottom = ##t
  ragged-last-bottom = ##t
}

global = {
  \key c \major
  \time 4/4
  \tempo "Moderato" 4 = 96
}

% Piccolo - concert pitch.
piccoloMusic = \relative c''' {
  \global
  R1*4
  \bar "|."
}

% Flute I - concert pitch.
fluteOneMusic = \relative c'' {
  \global
  R1*4
  \bar "|."
}

% Flute II - concert pitch.
fluteTwoMusic = \relative c'' {
  \global
  R1*4
  \bar "|."
}

% Oboe I - concert pitch.
oboeOneMusic = \relative c'' {
  \global
  R1*4
  \bar "|."
}

% Oboe II - concert pitch.
oboeTwoMusic = \relative c'' {
  \global
  R1*4
  \bar "|."
}

% English horn in F - concert pitch.
englishHornMusic = \relative c' {
  \global
  R1*4
  \bar "|."
}

% Clarinet I in B-flat - concert pitch.
clarinetOneMusic = \relative c' {
  \global
  R1*4
  \bar "|."
}

% Clarinet II in B-flat - concert pitch.
clarinetTwoMusic = \relative c' {
  \global
  R1*4
  \bar "|."
}

% Bass clarinet in B-flat - concert pitch.
bassClarinetMusic = \relative c {
  \global
  R1*4
  \bar "|."
}

% Bassoon I - concert pitch.
bassoonOneMusic = \relative c {
  \global
  R1*4
  \bar "|."
}

% Bassoon II - concert pitch.
bassoonTwoMusic = \relative c {
  \global
  R1*4
  \bar "|."
}

% Contrabassoon - concert pitch.
contrabassoonMusic = \relative c, {
  \global
  R1*4
  \bar "|."
}

% Horn I in F - concert pitch.
hornOneMusic = \relative c' {
  \global
  R1*4
  \bar "|."
}

% Horn II in F - concert pitch.
hornTwoMusic = \relative c' {
  \global
  R1*4
  \bar "|."
}

% Horn III in F - concert pitch.
hornThreeMusic = \relative c' {
  \global
  R1*4
  \bar "|."
}

% Horn IV in F - concert pitch.
hornFourMusic = \relative c' {
  \global
  R1*4
  \bar "|."
}

% Trumpet I in B-flat - concert pitch.
trumpetOneMusic = \relative c' {
  \global
  R1*4
  \bar "|."
}

% Trumpet II in B-flat - concert pitch.
trumpetTwoMusic = \relative c' {
  \global
  R1*4
  \bar "|."
}

% Trumpet III in B-flat - concert pitch.
trumpetThreeMusic = \relative c' {
  \global
  R1*4
  \bar "|."
}

% Trombone I - concert pitch.
tromboneOneMusic = \relative c {
  \global
  R1*4
  \bar "|."
}

% Trombone II - concert pitch.
tromboneTwoMusic = \relative c {
  \global
  R1*4
  \bar "|."
}

% Bass trombone - concert pitch.
bassTromboneMusic = \relative c {
  \global
  R1*4
  \bar "|."
}

% Tuba - concert pitch.
tubaMusic = \relative c {
  \global
  R1*4
  \bar "|."
}

% Timpani - concert pitch.
timpaniMusic = \relative c {
  \global
  R1*4
  \bar "|."
}

% Snare drum: replace rests with drum-mode notes, e.g. sn4 sn sn sn.
% The DrumStaff ignores the key signature in global.
snareMusic = \drummode {
  \global
  R1*4
  \bar "|."
}

% Harp right hand - concert pitch.
harpRightMusic = \relative c'' {
  \global
  R1*4
  \bar "|."
}

% Harp left hand - concert pitch.
harpLeftMusic = \relative c {
  \global
  R1*4
  \bar "|."
}

% Violins I - concert pitch.
violinOneMusic = \relative c'' {
  \global
  R1*4
  \bar "|."
}

% Violins II - concert pitch.
violinTwoMusic = \relative c'' {
  \global
  R1*4
  \bar "|."
}

% Violas - concert pitch.
violaMusic = \relative c' {
  \global
  R1*4
  \bar "|."
}

% Cellos - concert pitch.
celloMusic = \relative c {
  \global
  R1*4
  \bar "|."
}

% Double basses - concert pitch.
doubleBassMusic = \relative c {
  \global
  R1*4
  \bar "|."
}

scoreMusic = <<
  \new StaffGroup = "woodwinds" <<
    \new Staff = "piccolo" \with {
      instrumentName = "Piccolo"
      shortInstrumentName = "Picc."
      midiInstrument = "piccolo"
    } {
      \clef treble
      \transposition c''
      \transpose c' c \piccoloMusic
    }
    \new Staff = "fluteOne" \with {
      instrumentName = "Flute I"
      shortInstrumentName = "Fl. I"
      midiInstrument = "flute"
    } {
      \clef treble
      \fluteOneMusic
    }
    \new Staff = "fluteTwo" \with {
      instrumentName = "Flute II"
      shortInstrumentName = "Fl. II"
      midiInstrument = "flute"
    } {
      \clef treble
      \fluteTwoMusic
    }
    \new Staff = "oboeOne" \with {
      instrumentName = "Oboe I"
      shortInstrumentName = "Ob. I"
      midiInstrument = "oboe"
    } {
      \clef treble
      \oboeOneMusic
    }
    \new Staff = "oboeTwo" \with {
      instrumentName = "Oboe II"
      shortInstrumentName = "Ob. II"
      midiInstrument = "oboe"
    } {
      \clef treble
      \oboeTwoMusic
    }
    \new Staff = "englishHorn" \with {
      instrumentName = "English horn in F"
      shortInstrumentName = "E. hn."
      midiInstrument = "english horn"
    } {
      \clef treble
      \transposition f
      \transpose f c' \englishHornMusic
    }
    \new Staff = "clarinetOne" \with {
      instrumentName = \markup \concat { "Clarinet I in B" \flat }
      shortInstrumentName = "Cl. I"
      midiInstrument = "clarinet"
    } {
      \clef treble
      \transposition bes
      \transpose bes c' \clarinetOneMusic
    }
    \new Staff = "clarinetTwo" \with {
      instrumentName = \markup \concat { "Clarinet II in B" \flat }
      shortInstrumentName = "Cl. II"
      midiInstrument = "clarinet"
    } {
      \clef treble
      \transposition bes
      \transpose bes c' \clarinetTwoMusic
    }
    \new Staff = "bassClarinet" \with {
      instrumentName = \markup \concat { "Bass clarinet in B" \flat }
      shortInstrumentName = "B. cl."
      midiInstrument = "clarinet"
    } {
      \clef treble
      \transposition bes,
      \transpose bes, c' \bassClarinetMusic
    }
    \new Staff = "bassoonOne" \with {
      instrumentName = "Bassoon I"
      shortInstrumentName = "Bsn. I"
      midiInstrument = "bassoon"
    } {
      \clef bass
      \bassoonOneMusic
    }
    \new Staff = "bassoonTwo" \with {
      instrumentName = "Bassoon II"
      shortInstrumentName = "Bsn. II"
      midiInstrument = "bassoon"
    } {
      \clef bass
      \bassoonTwoMusic
    }
    \new Staff = "contrabassoon" \with {
      instrumentName = "Contrabassoon"
      shortInstrumentName = "Cbsn."
      midiInstrument = "bassoon"
    } {
      \clef bass
      \transposition c
      \transpose c c' \contrabassoonMusic
    }
  >>
  \new StaffGroup = "brass" <<
    \new Staff = "hornOne" \with {
      instrumentName = "Horn I in F"
      shortInstrumentName = "Hn. I"
      midiInstrument = "french horn"
    } {
      \clef treble
      \transposition f
      \transpose f c' \hornOneMusic
    }
    \new Staff = "hornTwo" \with {
      instrumentName = "Horn II in F"
      shortInstrumentName = "Hn. II"
      midiInstrument = "french horn"
    } {
      \clef treble
      \transposition f
      \transpose f c' \hornTwoMusic
    }
    \new Staff = "hornThree" \with {
      instrumentName = "Horn III in F"
      shortInstrumentName = "Hn. III"
      midiInstrument = "french horn"
    } {
      \clef treble
      \transposition f
      \transpose f c' \hornThreeMusic
    }
    \new Staff = "hornFour" \with {
      instrumentName = "Horn IV in F"
      shortInstrumentName = "Hn. IV"
      midiInstrument = "french horn"
    } {
      \clef treble
      \transposition f
      \transpose f c' \hornFourMusic
    }
    \new Staff = "trumpetOne" \with {
      instrumentName = \markup \concat { "Trumpet I in B" \flat }
      shortInstrumentName = "Tpt. I"
      midiInstrument = "trumpet"
    } {
      \clef treble
      \transposition bes
      \transpose bes c' \trumpetOneMusic
    }
    \new Staff = "trumpetTwo" \with {
      instrumentName = \markup \concat { "Trumpet II in B" \flat }
      shortInstrumentName = "Tpt. II"
      midiInstrument = "trumpet"
    } {
      \clef treble
      \transposition bes
      \transpose bes c' \trumpetTwoMusic
    }
    \new Staff = "trumpetThree" \with {
      instrumentName = \markup \concat { "Trumpet III in B" \flat }
      shortInstrumentName = "Tpt. III"
      midiInstrument = "trumpet"
    } {
      \clef treble
      \transposition bes
      \transpose bes c' \trumpetThreeMusic
    }
    \new Staff = "tromboneOne" \with {
      instrumentName = "Trombone I"
      shortInstrumentName = "Tbn. I"
      midiInstrument = "trombone"
    } {
      \clef bass
      \tromboneOneMusic
    }
    \new Staff = "tromboneTwo" \with {
      instrumentName = "Trombone II"
      shortInstrumentName = "Tbn. II"
      midiInstrument = "trombone"
    } {
      \clef bass
      \tromboneTwoMusic
    }
    \new Staff = "bassTrombone" \with {
      instrumentName = "Bass trombone"
      shortInstrumentName = "B. tbn."
      midiInstrument = "trombone"
    } {
      \clef bass
      \bassTromboneMusic
    }
    \new Staff = "tuba" \with {
      instrumentName = "Tuba"
      shortInstrumentName = "Tba."
      midiInstrument = "tuba"
    } {
      \clef bass
      \tubaMusic
    }
  >>
  \new StaffGroup = "percussion" <<
    \new Staff = "timpani" \with {
      instrumentName = "Timpani"
      shortInstrumentName = "Timp."
      midiInstrument = "timpani"
    } {
      \clef bass
      \timpaniMusic
    }
    \new DrumStaff = "snare" \with {
      instrumentName = "Snare drum"
      shortInstrumentName = "S. dr."
      drumStyleTable = #percussion-style
      \override StaffSymbol.line-count = #1
    } {
      \clef percussion
      \snareMusic
    }
  >>
  \new PianoStaff = "harp" \with {
    instrumentName = "Harp"
    shortInstrumentName = "Hp."
  } <<
    \new Staff = "harpRight" \with {
      midiInstrument = "orchestral harp"
    } {
      \clef treble
      \harpRightMusic
    }
    \new Staff = "harpLeft" \with {
      midiInstrument = "orchestral harp"
    } {
      \clef bass
      \harpLeftMusic
    }
  >>
  \new StaffGroup = "strings" <<
    \new Staff = "violinOne" \with {
      instrumentName = "Violins I"
      shortInstrumentName = "Vln. I"
      midiInstrument = "string ensemble 1"
    } {
      \clef treble
      \violinOneMusic
    }
    \new Staff = "violinTwo" \with {
      instrumentName = "Violins II"
      shortInstrumentName = "Vln. II"
      midiInstrument = "string ensemble 1"
    } {
      \clef treble
      \violinTwoMusic
    }
    \new Staff = "viola" \with {
      instrumentName = "Violas"
      shortInstrumentName = "Vla."
      midiInstrument = "string ensemble 1"
    } {
      \clef alto
      \violaMusic
    }
    \new Staff = "cello" \with {
      instrumentName = "Cellos"
      shortInstrumentName = "Vc."
      midiInstrument = "string ensemble 1"
    } {
      \clef bass
      \celloMusic
    }
    \new Staff = "doubleBass" \with {
      instrumentName = "Double basses"
      shortInstrumentName = "Cb."
      midiInstrument = "string ensemble 1"
    } {
      \clef bass
      \transposition c
      \transpose c c' \doubleBassMusic
    }
  >>
>>

\score {
  \scoreMusic
  \layout { }
  \midi {
    % Share channels by sound so full orchestral scores do not exhaust them.
    % Staves sharing a sound also share MIDI controllers (volume, pan, etc.).
    \context {
      \Score
      midiChannelMapping = #'instrument
    }
  }
}
