\version "2.24.3"
\language "english"
\include "articulate.ly"

\header {
  title = "CROSSCURRENT"
  subtitle = "a rhythmic miniature for solo piano"
  subsubtitle = "Incisive, buoyant, increasingly inevitable  |  ca. 83 seconds"
  composer = "Original composition · 2026"
  tagline = ##f
}

% Free serial construction, with deliberate repetitions and modal pedal tones.
% P0: C D G E B A | F# C# Eb Bb F Ab
% The two complementary hexachords suggest C and F# diatonic fields.
% Bars 19–22 explicitly superpose C Lydian and F# Mixolydian, exchanging hands.
% P6, I0, R0 and RI0 supply the subsequent melodic transformations.
% The opening six-note cell returns at bars 17, 33 and 37, progressively cut up.
% Durations at the printed tempi total 82.599 seconds, including the final rest.
% Eighth-note pulse is unchanged at the transition from 12/8 to 4/4.

#(set-global-staff-size 17)

\paper {
  #(set-paper-size "a4")
  top-margin = 11\mm
  bottom-margin = 11\mm
  left-margin = 13\mm
  right-margin = 13\mm
  indent = 10\mm
  short-indent = 0\mm
  ragged-last-bottom = ##f
  print-page-number = ##t
  system-system-spacing.basic-distance = #18
  system-system-spacing.minimum-distance = #12
  system-system-spacing.padding = #4
  markup-system-spacing.padding = #3
}

global = {
  \numericTimeSignature
  \accidentalStyle piano
  \time 12/8
  s1.*8
  \time 4/4
  s1*8
  \time 12/8
  s1.*12
  \time 7/8
  \set Timing.beatStructure = 2,2,3
  s8*7
  \time 5/8
  \set Timing.beatStructure = 2,3
  s8*5
  \time 7/8
  \set Timing.beatStructure = 2,2,3
  s8*7
  \time 5/8
  \set Timing.beatStructure = 2,3
  s8*5
  \time 12/8
  \set Timing.beatStructure = 3,3,3,3
  s1.*4
  \time 9/8
  \set Timing.beatStructure = 3,3,3
  s8*9
  \time 6/8
  s8*6
  \time 3/8
  s8*3
  \time 12/8
  \set Timing.beatStructure = 3,3,3,3
  s1.
}

right = {
  \clef treble
  \tempo "Vivo, elastico" 4. = 96
  \mark \markup \box "A · Spark"
  % 1–4: P0 and P6; the seven-attack timeline enters below.
  c''4->\mf ^\markup \italic "dry, bell-like; no pedal" d''4-- g''8-.
    e''4-> b''4-- a''4.-> |
  fs''4-> cs''4-- ef''8-. bf'4-> f''4-- af''4.-> |
  fs''4->\< af''4-- df''8-. bf'4-> f''4-- ef''4.-> |
  c''4-> g''4-- a''8-. e''4-> b''4-- d''4.->\! \break

  % 5–8: inversion, then retrograde; offbeat echoes disturb the balance.
  c''8->\f[ r bf'] f''4-> af''8-. cs''4-> ef''8-. r4. |
  fs''8->[ r b'] a''4-> d''8-. g''4-> e''8-. r4. |
  af''4->\> f''8-. bf''4-> ef''8-. df''4-> fs''8-. a''4-- b''8-. |
  e''4-> g''8-. d''4-> c''8-. r4. r4.\! \bar "||" \break

  % 9–12: displaced highlife-like chord responses; same eighth-note pulse.
  \tempo "Light, springing (same eighth)" 4 = 144
  \mark \markup \box "B · Tilt"
  r8\mp <e'' b''>8-. <d'' a''>4-> r8 <c'' g''>8~ q4 |
  r8 <ef'' bf''>8-. <df'' af''>4-> r8 <bf' f''>8~ q4 |
  r8\< <d'' g''>8-. <e'' a''>8-. r <g'' b''>4-> <e'' a''>8-. r |
  <ef'' af''>8->[ r <df'' f''>8] r <bf' ef''>8->[ <af' df''>8]
    <fs' bf'>4->\! \break

  % 13–16: RI0 in the upper voice; a quiet space before the interlock.
  e''8->\mf g''4-- d''8-. a''8-> b''8-. fs''4-- |
  ef''8-> cs''4-- af''8-. f''8-> bf''8-. c''4-- |
  <d'' g'' b''>8->\> r r <cs'' fs'' bf''>8->~ q4
    <c'' e'' a''>8-. r |
  <b' ef'' af''>8-> r <bf' d'' g''>4--\p r4 r4
    \bar "||" \pageBreak

  % 17–20: P0 split between the hands, with an emergent upper melody.
  \tempo "Flowing, interlocked" 4. = 96
  \mark \markup \box "C · Braid"
  c''8->\mp[ r g''-.] r b''->[ r] fs''->[ r ef''-.] r f''->[ r] |
  r8 d''-.[ r e''->] r a''-. r cs''->[ r bf'-.] r af''-> |
  c''8->[ e''-. g''-.] d''->[ a''-. e''-.]
    b''->[ g''-. a''-.] e''->[ fs''-. b'-.] \break
  fs''8->[ as''-. cs'''-.] gs''->[ ds'''-. as''-.]
    e'''->[ cs'''-. ds'''-.] as''->[ gs''-. b''-.] |

  % 21–24: complementary modal fields sound at once, then exchange hands.
  <c'' e''>4.\p( ^\markup \italic "sing above the mechanism"
    <d'' fs''>4 <e'' a''>8 <g'' b''>4. <e'' a''>4.) |
  <fs'' as''>4.( <gs'' b''>4 <as'' ds'''>8
    <cs''' e'''>4. <as'' ds'''>4.) \break
  c''8->\mf[ bf'-. f''-.] af''->[ cs''-. ef''-.]
    fs''->[ b'-. a''-.] d''->[ g''-. e''-.] |
  e''8->\<[ g''-. d''-.] a''->[ b''-. fs''-.]
    ef''->[ cs''-. af''-.] f''->[ bf''-. c'''-.]\! \bar "||"

  % 25–28: six quarter-note accents against four dotted-quarter bass beats.
  \tempo "Con fuoco" 4. = 104
  \mark \markup \box "D · Friction"
  <c'' g''>4->\f ^\markup \italic "six against four"
    <d'' a''>4-> <g' e''>4-> <e'' b''>4-> <b' fs''>4-> <a' e''>4-> \break
  <fs'' cs'''>4-> <cs'' af''>4-> <ef'' bf''>4->
    <bf' f''>4-> <f'' c'''>4-> <af' ef''>4-> |
  c''16->[\< d'' g'' e'' b'' a''] fs''16->[ cs'' ef'' bf' f'' af'']
    c'''8->[ g'' d''' e'''] b''->[ a''] |
  \ottava #1
  fs'''16->[ af''' df''' bf'' f''' ef'''] c'''16->[ g'' a'' e''' b'' d''']
    <fs'' bf''>8->[ <e'' a''> <ef'' af''>]
    <d'' g''>8->[ <cs'' fs''> <c'' f''>]\! \bar "||" \pageBreak

  % 29–32: subtractive cells; no tempo change at the changing metres.
  \mark \markup \box "E · Shear"
  c'''8->\ff[ d'''-.] g''->[ e'''-.] b''->[ a''-. fs''-.] |
  cs'''8->[ ef'''-.] bf''->[ f'''-. af''-.] |
  e'''8->[ g'''-.] d'''->[ a''-.] b''->[ fs''-. ef''-.] |
  cs'''8->[ af''-.] f'''->[ bf''-. c'''-.] \bar "||" \break

  % 33–36: recognisable return, harmonised; the bass answers in six.
  \ottava #0
  \mark \markup \box "F · Convergence"
  <c'' e''>4->\f <d'' g''>4-> <g'' b''>8-.
    <e'' a''>4-> <b'' d'''>4-> <a'' c'''>4.-> |
  <fs'' bf''>4-> <cs'' f''>4-> <ef'' af''>8-.
    <bf' ef''>4-> <f'' bf''>4-> <af'' df'''>4.-> |
  \ottava #1
  c'''8->\ff[ d''' g''] e'''->[ b'' a'']
    fs'''->[ cs''' ef'''] bf''->[ f''' af'''] \break
  af'''8->[ f''' bf''] ef'''->[ df''' fs''']
    a''->[ b'' e'''] g''->[ d''' c'''] |

  % 37–40: contraction, shared attacks, silence, then the final modal sonority.
  c'''4-> ^\markup \italic "relentless; do not broaden yet"
    d'''4-> g''8-. e'''4-> b''4-> \break
  a''4-> fs''4-> cs'''8-. ef'''8-> |
  <bf'' f'''>8-> r8 <af'' df'''>8-> |
  \ottava #0
  \tempo "Grave; one final breath" 4. = 88
  <e'' fs'' b'' d'''>2.\fff->-- ^\markup \italic "hold exactly; release together"
    r2. \bar "|."
}

left = {
  \clef bass
  % A: bass initially sparse; seven-attack bell cycle at bars 3–4.
  <c, g,>2.\mp r2. |
  r4. <fs, cs>4.-. r4. <fs, cs>4.-. |
  fs4->\mf cs'4-. fs8-. cs'4-> fs4-. cs'4-> fs8-. |
  c4-> g4-. c8-. g4-> c4-. g4-> c8-. |
  <c, g,>4.-> <ef, bf,>4.-> <fs, cs>4.-> <a, e>4.-> |
  <fs, cs>4.-> <a, e>4.-> <c g>4.-> <ef bf>4.-> |
  <fs, cs>4-> <af, ef>4-. <bf, f>8-. <df af>4-> <ef bf>4-. <f c'>4-> <fs cs'>8-. |
  <c g>4-> <d a>4-. <e b>8-. <g d'>4-> r4 r4. |

  % B: bass anticipations and open spaces beneath the displaced chords.
  c,8->\mp g,8-. r4 a,8-> r g,8-. r |
  fs,8-> cs8-. r4 ef8-> r cs8-. r |
  c,8-> r g,8-. d8-> r e8-. a,4-> |
  fs,8-> r cs8-. af,8-> r bf,8-. ef4-> |
  <c, g,>4.-> <fs, cs>8->~ q4 <a, e>4-. |
  <ef, bf,>4.-> <a, e>8->~ q4 <fs, cs>4-. |
  c,8-> r fs,4-. r8 a,8-. e4-> |
  ef,8-> r <c, g,>4-- r2 |

  % C: alternating attacks first trace the complete row across both hands.
  r8 d'-.[ r e'->] r a-. r cs'->[ r bf-.] r af-> |
  c'8->[ r g-.] r b->[ r] fs->[ r ef'-.] r f->[ r] |
  fs8->[ r cs'-.] gs->[ r ds'-.] as->[ r e'-.] b->[ r gs-.] |
  c8->[ r g-.] d->[ r a-.] e->[ r b-.] g->[ r fs-.] |
  fs8-.[\p cs' gs] ds'-.[ as e'] b-.[ gs ds'] as-.[ fs cs'] |
  c8-.[ g d'] a-.[ e' b] fs-.[ d' a] e'-.[ b g] |
  fs8->[ cs' af] ef'->[ bf f'] cs'->[ af ef'] bf->[ fs cs'] |
  c8->[ g d'] a->[ e' b] g->[ d' a] e'->[ b g] |

  % D: four dotted-quarter gestures, then a two-note bell ostinato.
  <c, g,>4.->\f <fs, cs>4.-> <a, e>4.-> <ef, bf,>4.-> |
  <fs, cs>4.-> <c, g,>4.-> <ef, bf,>4.-> <a, e>4.-> |
  c4-> g4-. c8-. g4-> c4-. g4-> c8-. |
  fs4-> cs'4-. fs8-. cs'4-> fs4-. cs'4-> fs8-. |

  % E: same pulse, fewer pulses; mutually displaced short cells.
  <c, g,>8-> r <fs, cs>8-> r <a, e>8-> r <ef, bf,>8-> |
  r8 <fs, cs>8-> <c, g,>8-> r <a, e>8-> |
  <ef, bf,>8-> r <a, e>8-> r <c, g,>8-> r <fs, cs>8-> |
  r8 <a, e>8-> <ef, bf,>8-> r <c, g,>8-> |

  % F: the countercycle broadens while the top line drives forward.
  <fs, cs>4-> <af, ef>4-> <bf, f>4-> <df af>4-> <ef bf>4-> <f c'>4-> |
  <c, g,>4-> <d, a,>4-> <e, b,>4-> <g, d>4-> <a, e>4-> <b, fs>4-> |
  <c, g,>4.-> <fs, cs>4.-> <ef, bf,>4.-> <a, e>4.-> |
  <af, ef>4.-> <f, c>4.-> <d, a,>4.-> <c, g,>4.-> |

  % Coda: both hands finally speak the same rhythm.
  <c, c>4-> <d, d>4-> <g, g>8-. <e, e>4-> <b,, b,>4-> |
  <a,, a,>4-> <fs, fs>4-> <cs, cs>8-. <ef, ef>8-> |
  <bf,, bf,>8-> r8 <af,, af,>8-> |
  <c, g, c>2.\fff->-- \sustainOn r2.\sustainOff |
}

pianoMusic = \new PianoStaff \with {
    instrumentName = "Piano"
    connectArpeggios = ##t
  } <<
    \new Staff = "upper" \with {
      midiInstrument = "acoustic grand"
      midiMinimumVolume = #0.30
      midiMaximumVolume = #0.92
    } << \global \right >>
    \new Staff = "lower" \with {
      midiInstrument = "acoustic grand"
      midiMinimumVolume = #0.22
      midiMaximumVolume = #0.82
    } << \global \left >>
  >>

\score {
  \pianoMusic
  \layout {
    \context {
      \Score
      \override RehearsalMark.self-alignment-X = #LEFT
      \override RehearsalMark.padding = #3
      \override MetronomeMark.padding = #3
    }
    \context {
      \PianoStaff
      \override StaffGrouper.staff-staff-spacing.basic-distance = #12
      \override StaffGrouper.staff-staff-spacing.minimum-distance = #10
    }
  }
}

\score {
  \articulate \pianoMusic
  \midi { }
}
