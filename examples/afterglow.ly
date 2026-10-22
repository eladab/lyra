\version "2.24.3"

\header {
  title = "Afterglow"
  subtitle = "for flute, violoncello and piano"
  composer = "Lyra"
  tagline = ##f
}

% An original, complete 24-bar miniature. Four systems on one A4 page.
% Both engraved notation and MIDI are available when opened in Lyra.
#(set-global-staff-size 15.5)

\paper {
  #(set-paper-size "a4")
  top-margin = 12\mm
  bottom-margin = 12\mm
  left-margin = 14\mm
  right-margin = 14\mm
  indent = 14\mm
  short-indent = 7\mm
  page-count = 1
  system-count = 4
  ragged-last-bottom = ##f
  print-page-number = ##f
  system-system-spacing.basic-distance = #16
  system-system-spacing.minimum-distance = #12
  system-system-spacing.padding = #3
  markup-system-spacing.padding = #4
}

global = {
  \key d \minor
  \time 6/8
}

flute = \relative c'' {
  \global
  \tempo "Andante con moto" 4. = 58

  \mark \default
  d4.\p( ^\markup \italic "dolce" a8 bes c) |
  f4.( e4 d8) |
  cis4( e8 a4 g8) |
  f4.( e4.) |
  d8(\< e f a4 g8) |
  e4.(\! cis4.) \break

  d4.( f8 e d) |
  a'4.( g4 f8) |
  e8(\< f g bes4 a8) |
  g4.(\! e4.) |
  f8(\> e d cis d e) |
  d2.\! \bar "||" \break

  \mark \default
  a'4.\mf( c8 bes a) |
  g4( a8 bes4 g8) |
  f4.( a4 c8) |
  bes4.( g4 e8) |
  a8( g f e f g) |
  e2.\> \bar "||" \break

  \mark \default
  d4.\p( a8 bes c) |
  f4.( e4 d8) |
  bes8( d f a4 g8) |
  e4( cis8 a4.) |
  d8(\> e f e d cis) |
  d2.\pp\fermata \bar "|."
}

cello = {
  \global
  \clef bass

  d4\p ^\markup \italic "pizz." a,8 d4 f8 |
  bes,4 f8 bes4 d'8 |
  a,4 e8 a4 cis'8 |
  d4 a,8 c4 a,8 |
  bes,4 d8 f4 g8 |
  a4 e8 a,4. |

  d4 a,8 d4 f8 |
  f,4 c8 f4 a8 |
  g,4 d8 g4 bes8 |
  c4 g,8 c4 e8 |
  bes,4 f8 a,4 e8 |
  d4 a,8 d4. |

  f4.\mf( ^\markup \italic "arco" a4 c'8) |
  e4( f8 g4 e8) |
  d4.( f4 a8) |
  c4.( e4 g8) |
  bes,4.( d4 g8) |
  a4.(\> cis'4 a8) |

  d4.\p( a,4.) |
  bes,4.( f4.) |
  g,4.( d4.) |
  a,4.( e4 cis8) |
  bes,4.(\> a,4.) |
  d,2.\pp\fermata
}

pianoRight = {
  \global
  \clef treble

  d'8\p( f' a' d'' a' f') |
  d'8( f' bes' d'' bes' f') |
  cis'8( e' a' cis'' a' e') |
  d'8( f' a' c'' a' f') |
  d'8( f' bes' d'' bes' f') |
  cis'8( e' a' cis'' a' e') |

  d'8( f' a' d'' a' f') |
  c'8( f' a' c'' a' f') |
  d'8( g' bes' d'' bes' g') |
  c'8( e' g' c'' g' e') |
  d'8( f' bes' cis' e' a') |
  d'8( f' a' d''4.) |

  <f' a' c''>4.\mf\arpeggio <a' c'' f''>4. |
  <e' g' bes'>8( c'' bes' g' bes' c'') |
  <d' f' a'>4.\arpeggio <f' a' d''>4. |
  <e' g' bes'>8( c'' bes' g' bes' c'') |
  <d' f' bes'>4.\arpeggio <d' g' bes'>4. |
  <cis' e' a'>8(\> e'' cis'' a' e' cis') |

  d'8\p( f' a' d'' a' f') |
  d'8( f' bes' d'' bes' f') |
  d'8( g' bes' d'' bes' g') |
  cis'8( e' a' cis'' a' e') |
  d'8(\> f' bes' cis' e' a') |
  <d' f' a' d''>2.\pp\arpeggio\fermata
}

pianoLeft = {
  \global
  \clef bass

  <d, d>4. <a,, a,>4. |
  <bes,, bes,>4. <f, f>4. |
  <a,, a,>4. <e, e>4. |
  <d, d>4. <c, c>4. |
  <bes,, bes,>4. <g,, g,>4. |
  <a,, a,>4. <e, e>4. |

  <d, d>4. <a,, a,>4. |
  <f, f>4. <c, c>4. |
  <g,, g,>4. <d, d>4. |
  <c, c>4. <g,, g,>4. |
  <bes,, bes,>4. <a,, a,>4. |
  <d, a, d>2. |

  <f, c f>4. <c, g, c>4. |
  <c, g, c>4. <e, bes, e>4. |
  <d, a, d>4. <a,, e, a,>4. |
  <c, g, c>4. <e, bes, e>4. |
  <bes,, f, bes,>4. <g,, d, g,>4. |
  <a,, e, a,>2. |

  <d, d>4. <a,, a,>4. |
  <bes,, bes,>4. <f, f>4. |
  <g,, g,>4. <d, d>4. |
  <a,, a,>4. <e, e>4. |
  <bes,, bes,>4. <a,, a,>4. |
  <d, a, d>2.\fermata
}

\score {
  \new StaffGroup <<
    \new Staff \with {
      instrumentName = "Flute"
      shortInstrumentName = "Fl."
      midiInstrument = "flute"
    } \flute
    \new Staff \with {
      instrumentName = "Cello"
      shortInstrumentName = "Vc."
      midiInstrument = "cello"
    } \cello
    \new PianoStaff \with {
      instrumentName = "Piano"
      shortInstrumentName = "Pno."
    } <<
      \new Staff \with {
        midiInstrument = "acoustic grand"
      } \pianoRight
      \new Staff \with {
        midiInstrument = "acoustic grand"
      } \pianoLeft
    >>
  >>
  \layout {
    \context {
      \Score
      \override BarNumber.font-size = #-1
      \override RehearsalMark.self-alignment-X = #LEFT
    }
    \context {
      \Staff
      \override VerticalAxisGroup.staff-staff-spacing.basic-distance = #10
    }
  }
  \midi { \tempo 4. = 58 \volume 127 }
}
