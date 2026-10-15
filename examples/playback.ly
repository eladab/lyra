\version "2.24.3"
\header { title = "Lyra playback" }
\score {
  \relative c' {
    \key c \major
    \time 4/4
    c4 d e f | g a b c | c b a g | f e d c | c1 \bar "|."
  }
  \layout { }
  \midi { \tempo 4 = 100 }
}
