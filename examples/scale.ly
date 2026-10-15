\version "2.24.3"
\header { title = "First steps with Lyra" }
\score {
  \relative c' {
    c4 d e f | g a b c | c b a g | f e d c | c d e f | g a b c | c b a g | f e d a | c1 \bar "|."
  }
  {
    c4 d e f | g a b c | c b a g | f e d c | c d e f | g a b c | c b a g | f e d a | c1 \bar "|."
  }

  \layout { }
  \midi { \tempo 4 = 100 }
}
