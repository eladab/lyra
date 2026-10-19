\version "2.24.3"

% Blank Bach chorale in the traditional four-voice keyboard reduction:
% treble staff: soprano (stems up), alto (stems down);
% bass staff: tenor (stems up), bass (stems down).
% The sixteen measures contain only invisible spacers, not notes or rests.
% Replace the s1 spacers with music, keeping all four voices equal in length.
% Use \fermata on the closing note of each chorale phrase.

\header {
  title = "CHORAL"
  composer = "J. S. Bach"
  tagline = ##f
}

#(set-global-staff-size 18)

\paper {
  #(set-paper-size "a4")
  top-margin = 15\mm
  bottom-margin = 18\mm
  left-margin = 18\mm
  right-margin = 18\mm
  indent = 0\mm
  page-count = 1
  ragged-last-bottom = ##f
  print-page-number = ##f
  system-system-spacing.basic-distance = #24
  system-system-spacing.minimum-distance = #18
  system-system-spacing.padding = #6
}

global = {
  % Change the key and meter here for the chorale you are entering.
  \key c \major
  \time 4/4
}

soprano = \relative c'' {
  \global
  s1 | s1 | s1 | s1 | \break
  s1 | s1 | s1 | s1 | \break
  s1 | s1 | s1 | s1 | \break
  s1 | s1 | s1 | s1 \bar "|."
}

alto = \relative c' {
  \global
  s1 | s1 | s1 | s1 |
  s1 | s1 | s1 | s1 |
  s1 | s1 | s1 | s1 |
  s1 | s1 | s1 | s1 |
}

tenor = \relative c' {
  \global
  s1 | s1 | s1 | s1 |
  s1 | s1 | s1 | s1 |
  s1 | s1 | s1 | s1 |
  s1 | s1 | s1 | s1 |
}

bass = \relative c {
  \global
  s1 | s1 | s1 | s1 |
  s1 | s1 | s1 | s1 |
  s1 | s1 | s1 | s1 |
  s1 | s1 | s1 | s1 |
}

choraleText = \lyricmode {
  % Type your lyrics on the blank line below. No note durations are needed.
  % Use -- to split syllables, e.g. Freu -- de.
  % Lyrics follow the soprano and appear once it has notes instead of s1.

}

\score {
  \new PianoStaff <<
    \new Staff = "upper" \with {
      midiInstrument = "acoustic grand"
    } <<
      \clef treble
      \new Voice = "soprano" { \voiceOne \soprano }
      \new Voice = "alto" { \voiceTwo \alto }
    >>

    \new Lyrics \with {
      \override LyricText.font-family = #'serif
      \override LyricText.font-size = #-1
    } \lyricsto "soprano" \choraleText

    \new Staff = "lower" \with {
      midiInstrument = "acoustic grand"
    } <<
      \clef bass
      \new Voice = "tenor" { \voiceOne \tenor }
      \new Voice = "bass" { \voiceTwo \bass }
    >>
  >>

  \layout {
    \context {
      \Score
      \omit BarNumber
      \override SpacingSpanner.uniform-stretching = ##t
    }
  }

  % Silent until notes are entered; all four parts use piano playback.
  \midi { \tempo 4 = 72 }
}
