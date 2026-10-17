{
	\clef bass
	c'4 b g, g, |
	g, f, f, c' |
}

{
	\key a \major
	\time 6/8
	cis''8. d''16 cis''8 e''4 e''8 |
	b'8. cis''16 b'8 d''4 d''8
}

\fixed c'' {
	\key a \major
	\time 6/8
	cis8. d16 cis8 e4 e8 |
	b,8. cis16 b,8 d4 d8
}

\relative {
	\key a \major
	\time 6/8
	cis''8. d16 cis8 e4 e8 |
	b8. cis16 b8 d4 d8
}

% relative inside relative
\relative { c'4 \relative { f'' g } c }

% fixed in side relative
\relative {
	c'4 \fixed c { f'' g'' } c |
	c4 \fixed c'' { f g } c
}
					

