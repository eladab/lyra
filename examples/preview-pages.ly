\version "2.24.3"

% Two distinct pages for checking navigation, zoom and preview clipping.
\book {
  \bookpart {
    \header { title = "Lyra preview — page one" }
    \score {
      \relative c' {
        \key c \major
        \time 4/4
        c4 d e f | g a b c | c1 \bar "|."
      }
      \layout { }
    }
  }
  \bookpart {
    \header { title = "Lyra preview — page two" }
    \score {
      \relative c'' {
        \key g \major
        \time 3/4
        g4 b d | c b a | g2. \bar "|."
      }
      \layout { }
    }
  }
}
