% Chamber winds - Clarinet quintet
% Source: Lyra ensemble library
% Original Lyra starter, not an excerpt from the LilyPond manual.
% Instrumentation: Clarinet in B-flat, 2 violins, viola, cello.
% Enter all pitched music at CONCERT pitch in the *Music variables below.
% The score converts it to written pitch and declares each MIDI transposition.
% Reference: https://lilypond.org/doc/v2.26/Documentation/notation/displaying-pitches#instrument-transpositions
% Replace each R1*4 with your music; keep all parts the same total duration.
% Change the shared key, meter and tempo in global. R1*4 is four bars in 4/4.
% This variant uses B-flat clarinet. For an A clarinet, change the labels,
% transposition bes to a, and transpose bes c' to transpose a c'.
\version "2.26.0"

#(set-global-staff-size 18)

\header {
  title = "Untitled"
  subtitle = "Clarinet quintet"
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

% Clarinet in B-flat - concert pitch.
clarinetMusic = \relative c' {
  \global
  R1*4
  \bar "|."
}

% Violin I - concert pitch.
violinOneMusic = \relative c'' {
  \global
  R1*4
  \bar "|."
}

% Violin II - concert pitch.
violinTwoMusic = \relative c'' {
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
  \new StaffGroup = "ensemble" <<
    \new Staff = "clarinet" \with {
      instrumentName = \markup \concat { "Clarinet in B" \flat }
      shortInstrumentName = "Cl."
      midiInstrument = "clarinet"
    } {
      \clef treble
      \transposition bes
      \transpose bes c' \clarinetMusic
    }
    \new Staff = "violinOne" \with {
      instrumentName = "Violin I"
      shortInstrumentName = "Vln. I"
      midiInstrument = "violin"
    } {
      \clef treble
      \violinOneMusic
    }
    \new Staff = "violinTwo" \with {
      instrumentName = "Violin II"
      shortInstrumentName = "Vln. II"
      midiInstrument = "violin"
    } {
      \clef treble
      \violinTwoMusic
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
