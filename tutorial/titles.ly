myWidth = 60      % a number to pass to a \paper variable
                  % (the unit is millimeter)
myName = "Wendy"  % a string to pass to a markup
aFivePaper = \paper { #(set-paper-size "a5") }
\paper {
  \aFivePaper
  line-width = \myWidth
}

{
  c4^\myName
}
