% Chamber strings - String trio
% Source: Lyra ensemble library
% Original Lyra starter, not an excerpt from the LilyPond manual.
% Instrumentation: Violin, viola, cello.
% Enter all pitched music at CONCERT pitch in the *Music variables below.
% The score converts it to written pitch and declares each MIDI transposition.
% Reference: https://lilypond.org/doc/v2.26/Documentation/notation/displaying-pitches#instrument-transpositions
% Replace each R1*4 with your music; keep all parts the same total duration.
% Change the shared key, meter and tempo in global. R1*4 is four bars in 4/4.
\version "2.26.0"

#(set-global-staff-size 18)

\header {
  title = "Untitled"
  subtitle = "String trio"
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

% Violin - concert pitch.
violinMusic = \relative c'' {
  \global
  R1*4
  \bar "|."
}

% Viola - concert pitch.
violaMusic = \relative c' {
  \global
  R1*4
  \bar "|."
}

% Cello - concert pitch.
celloMusic = \relative c {
  \global
  R1*4
  \bar "|."
}

scoreMusic = <<
  \new StaffGroup = "strings" <<
    \new Staff = "violin" \with {
      instrumentName = "Violin"
      shortInstrumentName = "Vln."
      midiInstrument = "violin"
    } {
      \clef treble
      \violinMusic
    }
    \new Staff = "viola" \with {
      instrumentName = "Viola"
      shortInstrumentName = "Vla."
      midiInstrument = "viola"
    } {
      \clef alto
      \violaMusic
    }
    \new Staff = "cello" \with {
      instrumentName = "Cello"
      shortInstrumentName = "Vc."
      midiInstrument = "cello"
    } {
      \clef bass
      \celloMusic
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
