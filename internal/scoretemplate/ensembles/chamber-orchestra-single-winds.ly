% Orchestral - Chamber orchestra (single winds)
% Source: Lyra ensemble library
% Original Lyra starter, not an excerpt from the LilyPond manual.
% Instrumentation: 1 flute, 1 oboe, 1 clarinet, 1 bassoon; 2 horns; strings.
% Enter all pitched music at CONCERT pitch in the *Music variables below.
% The score converts it to written pitch and declares each MIDI transposition.
% Reference: https://lilypond.org/doc/v2.26/Documentation/notation/displaying-pitches#instrument-transpositions
% Replace each R1*4 with your music; keep all parts the same total duration.
% Change the shared key, meter and tempo in global. R1*4 is four bars in 4/4.
% String staves represent sections; choose the number of players separately.
% Section strings share a General MIDI ensemble sound.
\version "2.26.0"

#(set-global-staff-size 15)

\header {
  title = "Untitled"
  subtitle = "Chamber orchestra (single winds)"
  composer = "Composer"
  tagline = ##f
}

\paper {
  #(set-paper-size "a4")
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

% Flute - concert pitch.
fluteMusic = \relative c'' {
  \global
  R1*4
  \bar "|."
}

% Oboe - concert pitch.
oboeMusic = \relative c'' {
  \global
  R1*4
  \bar "|."
}

% Clarinet in B-flat - concert pitch.
clarinetMusic = \relative c' {
  \global
  R1*4
  \bar "|."
}

% Bassoon - concert pitch.
bassoonMusic = \relative c {
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
    \new Staff = "flute" \with {
      instrumentName = "Flute"
      shortInstrumentName = "Fl."
      midiInstrument = "flute"
    } {
      \clef treble
      \fluteMusic
    }
    \new Staff = "oboe" \with {
      instrumentName = "Oboe"
      shortInstrumentName = "Ob."
      midiInstrument = "oboe"
    } {
      \clef treble
      \oboeMusic
    }
    \new Staff = "clarinet" \with {
      instrumentName = \markup \concat { "Clarinet in B" \flat }
      shortInstrumentName = "Cl."
      midiInstrument = "clarinet"
    } {
      \clef treble
      \transposition bes
      \transpose bes c' \clarinetMusic
    }
    \new Staff = "bassoon" \with {
      instrumentName = "Bassoon"
      shortInstrumentName = "Bsn."
      midiInstrument = "bassoon"
    } {
      \clef bass
      \bassoonMusic
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
