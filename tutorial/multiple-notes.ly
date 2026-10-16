<<
	\relative { a'2 g }
	\relative { f'2 e }
	\relative { d'2 b }
>>

\relative {
	c''2 <<c e>> |
	<< { e2 f } { c2 <<b d>> } >> |
}

<<
	\new Staff { \clef treble c''4 }
	\new Staff { \clef bass c4 }
>>

<<
	\new Staff { \clef treble \key d \major \time 3/4 c''4 }
	\new Staff { \clef bass c4 }
>>

\new PianoStaff <<
	\new Staff \relative { \time 2/4 c''4 e | g g, | }
	\new Staff \relative { \clef bass c4 c' | e c | }
>>
