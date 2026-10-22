% Chamber winds - Woodwind quintet
% Source: Lyra ensemble library
% Original Lyra starter, not an excerpt from the LilyPond manual.
% Instrumentation: Flute, oboe, clarinet in B-flat, horn in F, bassoon.
% Enter all pitched music at CONCERT pitch in the *Music variables below.
% The score converts it to written pitch and declares each MIDI transposition.
% Reference: https://lilypond.org/doc/v2.26/Documentation/notation/displaying-pitches#instrument-transpositions
% Replace each R1*4 with your music; keep all parts the same total duration.
% Change the shared key, meter and tempo in global. R1*4 is four bars in 4/4.
\version "2.26.0"

#(set-global-staff-size 18)

\header {
  title = "Untitled"
  subtitle = "Woodwind quintet"
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

% Horn in F - concert pitch.
hornMusic = \relative c' {
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

scoreMusic = <<
  \new StaffGroup = "winds" <<
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
    \new Staff = "horn" \with {
      instrumentName = "Horn in F"
      shortInstrumentName = "Hn."
      midiInstrument = "french horn"
    } {
      \clef treble
      \transposition f
      \transpose f c' \hornMusic
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
