% Orchestral - String orchestra
% Source: Lyra ensemble library
% Original Lyra starter, not an excerpt from the LilyPond manual.
% Instrumentation: Violins I/II, violas, cellos, double basses.
% Enter all pitched music at CONCERT pitch in the *Music variables below.
% The score converts it to written pitch and declares each MIDI transposition.
% Reference: https://lilypond.org/doc/v2.26/Documentation/notation/displaying-pitches#instrument-transpositions
% Replace each R1*4 with your music; keep all parts the same total duration.
% Change the shared key, meter and tempo in global. R1*4 is four bars in 4/4.
% String staves represent sections; choose the number of players separately.
% Section strings share a General MIDI ensemble sound.
\version "2.26.0"

#(set-global-staff-size 18)

\header {
  title = "Untitled"
  subtitle = "String orchestra"
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
