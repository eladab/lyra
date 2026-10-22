\version "2.26.0"
\language "english"

% Source: /Users/elad/Downloads/tikvar.mp3, 3:04.000 to EOF (3:54.475).
% Best-effort audio-derived transcription, edited as a solo-piano reduction.
% The first attack is at 3:04.462; the last is at 3:45.332.
% Quarter-note pulse and barlines are editorial. The last gesture is in 5/4.
% Melody and bass registers are retained; selected inner voices are moved by
% octaves or omitted for two hands. See tikvar-piano.md for uncertainties.
% English pitch names: cs = C-sharp, ef = E-flat, bf = B-flat.

\header {
  title = "Tikvar"
  subtitle = "Closing excerpt · solo piano"
  subsubtitle = "3:04 to end · best-effort transcription and reduction"
  composer = "From the supplied recording"
  arranger = "Transcribed for Lyra"
  tagline = ##f
}

#(set-global-staff-size 18)
\paper {
  #(set-paper-size "a4")
  top-margin = 12\mm
  bottom-margin = 13\mm
  left-margin = 14\mm
  right-margin = 14\mm
  indent = 0\mm
  short-indent = 0\mm
  page-count = 1
  system-count = 4
  ragged-last-bottom = ##f
  print-page-number = ##f
  system-system-spacing.basic-distance = #20
  system-system-spacing.minimum-distance = #15
  system-system-spacing.padding = #4
  markup-system-spacing.padding = #4
}

form = {
  \numericTimeSignature
  \time 4/4
  \partial 8 s8
  s1*7
  \time 5/4 s1*5/4
  \time 4/4 s1
}

rightHand = {
  \clef treble
  \tag #'print { \tempo "Lento, rubato" 4 = 50 }
  r8
  % m. 1 -- source 3:04.462. The opening silence is an approximate pickup rest.
  d'4\mp ^\markup \italic "cantabile" fs' gs' c'' |
  % m. 2 -- 3:09.330
  e''4 d'' cs'' b' |
  % m. 3 -- 3:14.272
  a'4 <e' g' cs''> <fs' ds''> <e' g'> \break

  % m. 4 -- 3:19.050. Roll the wide right-hand intervals lightly.
  <a' b''>4\mf\arpeggio ^\markup \italic "roll wide intervals"
  <fs' a''>\arpeggio <gs' gs''> <c'' fs''> |
  % m. 5 -- 3:23.503. Inner voices are redistributed around the melody.
  <c'' e''>4 <gs' d'' gs''> <cs'' fs'' bf''>
  \ottava #1 <a'' b'' d'''> \break

  % m. 6 -- 3:28.474. Preserve the high melody; lighten the inner texture.
  <a'' cs''' fs'''>4\f <a'' cs''' e'''>
  <ds'' ds'''>\arpeggio <g'' cs'''> |
  % m. 7 -- 3:34.142
  <c'' b''>4 <ds'' a''> <fs'' gs''>
  \ottava #0 <a' c'' fs''> \break

  % m. 8 -- 3:39.147. Editorial 5/4 accommodates the slowing final gesture.
  <cs'' e''>4\mf ^\markup \italic "rit., freely" d'
  <e' d''>4.\p\arpeggio f'4. |
  % m. 9 -- 3:45.332. Keep the resonance through the recorded decay.
  r1\fermata \bar "|."
}

leftHand = {
  \clef treble
  r8
  % m. 1
  r4 c'\mp b a |
  % m. 2
  g4 b cs' f' |
  % m. 3
  d'4 bf gs \clef bass e |
  % m. 4
  c4\mp d <e b ds'> <f b> |
  % m. 5
  <g b>4 <ds a ds'> <cs b> <a, c d> |
  % m. 6 -- octave-reduced inner voices; all bass roots retained.
  <f, bf, d>4\mf <g, bf, d> <a, e g gs>\arpeggio <bf, e f gs> |
  % m. 7
  <c b>4 <d gs> <ef bf> <f d'> |
  % m. 8
  <d g bf>4 <d gs> <a, a>4.\arpeggio r4. |
  % m. 9
  d1\pp\fermata
}

piano = \new PianoStaff \with {
  \consists "Span_arpeggio_engraver"
  connectArpeggios = ##t
} <<
  \new Staff = "right" \with { midiInstrument = "acoustic grand" }
    << \form \rightHand >>
  \new Staff = "left" \with { midiInstrument = "acoustic grand" }
    << \form \leftHand >>
>>

\score {
  \keepWithTag #'print \piano
  \layout {
    \context { \Score \accidentalStyle piano }
    \context { \PianoStaff \override StaffGrouper.staff-staff-spacing.basic-distance = #12 }
  }
}

\markup \vspace #0.6
\markup \small \column {
  \line { \italic "Light pedal, changing with the harmony; let the final bass note fade." }
  \line { "Barlines, voicing and dynamics are editorial. Some pitches remain uncertain." }
}

% Approximate the measured attack spacing in playback without cluttering the
% printed page. These tempo changes follow the source's rubato, not a click.
performanceTiming = {
  \tempo 4 = 65 s8
  \tempo 4 = 48 s4
  \tempo 4 = 49 s4
  \tempo 4 = 50 s4
  \tempo 4 = 51 s4
  \tempo 4 = 50 s4
  \tempo 4 = 49 s4
  \tempo 4 = 48 s4
  \tempo 4 = 47 s4
  \tempo 4 = 46 s4
  \tempo 4 = 48 s4
  \tempo 4 = 50 s4
  \tempo 4 = 58 s4
  \tempo 4 = 45 s4
  \tempo 4 = 53 s4
  \tempo 4 = 53 s4
  \tempo 4 = 72 s4
  \tempo 4 = 40 s4
  \tempo 4 = 49 s4
  \tempo 4 = 47 s4
  \tempo 4 = 62 s4
  \tempo 4 = 36 s4
  \tempo 4 = 68 s4
  \tempo 4 = 32 s4
  \tempo 4 = 48 s4
  \tempo 4 = 41 s4
  \tempo 4 = 50 s4
  \tempo 4 = 52 s4
  \tempo 4 = 51 s4
  \tempo 4 = 46 s4
  \tempo 4 = 55 s4
  \tempo 4 = 44 s4.
  \tempo 4 = 52 s4.
  \tempo 4 = 40 s1
}

\score {
  <<
    \removeWithTag #'print \piano
    \new Devnull { \performanceTiming }
  >>
  \midi { }
}
