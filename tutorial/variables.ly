myWidth = 120
myName = "Wendy"
aFivePaper = \paper { #(set-paper-size "a5") }

\paper {
	\aFivePaper
	line-width = \myWidth
}

violin = \new Staff{
	\relative { a''4 b c b }
}

cello = \new Staff{
	\relative {
		\clef bass
		e2 d
	}
}

{
	<<
		\violin
		\cello
	>>
}

tripletA = \relative { \tuplet 3/2 { c'8 e g } }
barA = \relative { \tripletA \tripletA \tripletA \tripletA }

{ \barA \barA }
