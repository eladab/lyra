% Chamber brass - Brass quintet
% Source: Lyra ensemble library
% Original Lyra starter, not an excerpt from the LilyPond manual.
% Instrumentation: 2 trumpets in B-flat, horn in F, trombone, tuba.
% Enter all pitched music at CONCERT pitch in the *Music variables below.
% The score converts it to written pitch and declares each MIDI transposition.
% Reference: https://lilypond.org/doc/v2.26/Documentation/notation/displaying-pitches#instrument-transpositions
% Replace each R1*4 with your music; keep all parts the same total duration.
% Change the shared key, meter and tempo in global. R1*4 is four bars in 4/4.
\version "2.26.0"

#(set-global-staff-size 18)

\header {
  title = "Untitled"
  subtitle = "Brass quintet"
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

% Horn in F - concert pitch.
hornMusic = \relative c' {
  \global
  R1*4
  \bar "|."
}

% Trombone - concert pitch.
tromboneMusic = \relative c {
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

scoreMusic = <<
  \new StaffGroup = "brass" <<
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
    \new Staff = "horn" \with {
      instrumentName = "Horn in F"
      shortInstrumentName = "Hn."
      midiInstrument = "french horn"
    } {
      \clef treble
      \transposition f
      \transpose f c' \hornMusic
    }
    \new Staff = "trombone" \with {
      instrumentName = "Trombone"
      shortInstrumentName = "Tbn."
      midiInstrument = "trombone"
    } {
      \clef bass
      \tromboneMusic
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
