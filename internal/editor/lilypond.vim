" Lyra's session-local LilyPond color layer. Only Lyra-named groups are defined;
" the user's colorscheme, ftplugins, options and mappings stay in charge.
let s:cpo_save = &cpo
set cpo&vim

function! s:Palette() abort
  highlight LyraLilyCommand        guifg=#b5a4e8 ctermfg=146 gui=NONE cterm=NONE
  highlight LyraLilyStructure      guifg=#b5a4e8 ctermfg=146 gui=bold cterm=bold
  highlight LyraLilyNote           guifg=#82c9c1 ctermfg=116 gui=NONE cterm=NONE
  highlight LyraLilyOctave         guifg=#a2d6cf ctermfg=152 gui=NONE cterm=NONE
  highlight LyraLilyDuration       guifg=#d7bd81 ctermfg=180 gui=NONE cterm=NONE
  highlight LyraLilyRest           guifg=#d99aaa ctermfg=175 gui=NONE cterm=NONE
  highlight LyraLilyDynamic        guifg=#e3a8b7 ctermfg=181 gui=bold cterm=bold
  highlight LyraLilyArticulation   guifg=#90b7dc ctermfg=110 gui=NONE cterm=NONE
  highlight LyraLilyDelimiter      guifg=#8fa6b8 ctermfg=109 gui=NONE cterm=NONE
  highlight LyraLilyProperty       guifg=#a8c8e8 ctermfg=153 gui=NONE cterm=NONE
  highlight LyraLilyString         guifg=#b8c99d ctermfg=150 gui=NONE cterm=NONE
  highlight LyraLilyComment        guifg=#82919e ctermfg=102 gui=italic cterm=italic
  highlight LyraLilyBlockComment   guifg=#82919e ctermfg=102 gui=italic cterm=italic
  highlight LyraLilyMarkup         guifg=#d8d0bd ctermfg=187 gui=NONE cterm=NONE
  highlight LyraLilyLyrics         guifg=#b8c99d ctermfg=150 gui=NONE cterm=NONE
  highlight LyraLilyScheme         guifg=#d3b895 ctermfg=180 gui=NONE cterm=NONE
  highlight LyraLilySchemeSymbol   guifg=#d9a6bc ctermfg=182 gui=NONE cterm=NONE
  highlight LyraLilySchemeNumber   guifg=#d7bd81 ctermfg=180 gui=NONE cterm=NONE
  highlight LyraLilySchemeBoolean  guifg=#d99aaa ctermfg=175 gui=NONE cterm=NONE
  highlight LyraLilySchemeFunction guifg=#b5a4e8 ctermfg=146 gui=NONE cterm=NONE
  highlight LyraLilySchemeComment  guifg=#82919e ctermfg=102 gui=italic cterm=italic
  highlight link LyraLilySchemeString LyraLilyString
  highlight link LyraLilySchemeBlockComment LyraLilySchemeComment
endfunction

function! s:IsLilyPond() abort
  return &l:buftype ==# '' && (&l:filetype ==# 'lilypond' || expand('%:e') =~? '^i\?ly$')
endfunction

function! s:Apply(force) abort
  if !s:IsLilyPond() || (!a:force && get(b:, 'lyra_lilypond_syntax', 0))
    return
  endif
  " A transparent document region adds our vocabulary without clearing any
  " existing syntax definitions or changing shared groups such as Identifier.
  " Its explicit contents keep notes out of prose, strings and embedded Scheme.
  syntax cluster LyraLilyScore contains=LyraLilyCommand,LyraLilyStructure,LyraLilyNote,LyraLilyOctave,LyraLilyDuration,LyraLilyRest,LyraLilyDynamic,LyraLilyArticulation,LyraLilyDelimiter,LyraLilyProperty,LyraLilyString,LyraLilyComment,LyraLilyBlockComment,LyraLilyMarkup,LyraLilyLyrics,LyraLilyScheme,LyraLilySchemeSymbol,LyraLilySchemeBoolean
  syntax cluster LyraLilyProse contains=LyraLilyCommand,LyraLilyStructure,LyraLilyProperty,LyraLilyString,LyraLilyComment,LyraLilyBlockComment,LyraLilyMarkupBrace,LyraLilyScheme,LyraLilySchemeSymbol,LyraLilySchemeNumber,LyraLilySchemeBoolean
  syntax cluster LyraLilySchemeItems contains=LyraLilySchemeParen,LyraLilySchemeString,LyraLilySchemeSymbol,LyraLilySchemeNumber,LyraLilySchemeBoolean,LyraLilySchemeFunction,LyraLilySchemeComment,LyraLilySchemeBlockComment,LyraLilyEmbeddedMusic

  syntax match LyraLilyCommand /\\[A-Za-z][A-Za-z0-9_-]*/ contained
  syntax match LyraLilyStructure /\\\%(score\|relative\|absolute\|fixed\|new\|context\|header\|paper\|layout\|midi\|with\|parallelMusic\|simultaneous\|sequential\|repeat\|alternative\|tuplet\|lyrics\|lyricmode\|lyricsto\|chordmode\|drummode\|figuremode\|markup\|markuplist\)\>/ contained
  syntax match LyraLilyNote /\%([[:alnum:]_]\)\@<!\%(do\|re\|mi\|fa\|sol\|la\|si\|ut\|[a-h]\)\%(isis\|eses\|isih\|eseh\|is\|es\|ih\|eh\|ss\|ff\|s\|f\)\?\ze\%([0-9',!?]\|\>\)/ contained
  syntax match LyraLilyOctave /[',]\+/ contained
  syntax match LyraLilyDuration /\d\+\.*\|\\\%(breve\|longa\|maxima\)\>/ contained
  syntax match LyraLilyRest /\<[rRs]\ze\%(\d\|\>\)/ contained
  syntax match LyraLilyDynamic /\\\%(p\{1,5}\|f\{1,5}\|mp\|mf\|fp\|sf\|sff\|sfz\|sffz\|sfp\|sfpp\|rfz\|fz\|cresc\|decresc\|dim\)\>/ contained
  syntax match LyraLilyArticulation /[-_^][-_^+.|>]\|[~()]\|[][]\|\\[<>!()]/ contained
  syntax match LyraLilyDelimiter +[{}<>|=:/*]+ contained
  syntax match LyraLilyProperty /[A-Za-z_][A-Za-z0-9_.-]*\ze\s*=/ contained
  syntax region LyraLilyString start=/"/ skip=/\\./ end=/"/ contained contains=@Spell
  syntax match LyraLilyComment /%.*$/ contained contains=@Spell
  syntax region LyraLilyBlockComment start=/%{/ end=/%}/ contained contains=LyraLilyBlockComment,@Spell

  syntax region LyraLilyMarkupBrace transparent matchgroup=LyraLilyDelimiter start=/{/ end=/}/ contained contains=@LyraLilyProse
  syntax region LyraLilyMarkup matchgroup=LyraLilyStructure start=/\\\%(markup\|markuplist\)\>\%(\s*\\[A-Za-z][A-Za-z0-9_-]*\)*\s*{/ end=/}/ contained contains=@LyraLilyProse
  syntax region LyraLilyLyrics matchgroup=LyraLilyStructure start=/\\\%(lyrics\|lyricmode\)\>\s*{/ end=/}/ contained contains=@LyraLilyProse

  syntax match LyraLilySchemeSymbol /#['`]\?[A-Za-z_][A-Za-z0-9_:+*\/!?<>=.-]*/ contained
  syntax match LyraLilySchemeNumber /#\?[-+]\?\d\+\%([./]\d\+\)\?/ contained
  syntax match LyraLilySchemeBoolean /#\{1,2}[tf]\>/ contained
  syntax match LyraLilySchemeFunction /\%((\s*\)\@<=[A-Za-z_+*\/!?<>=-][A-Za-z0-9_:+*\/!?<>=.-]*/ contained
  syntax region LyraLilySchemeString start=/"/ skip=/\\./ end=/"/ contained
  syntax match LyraLilySchemeComment /;.*$/ contained
  syntax region LyraLilySchemeBlockComment start=/#|/ end=/|#/ contained contains=LyraLilySchemeBlockComment
  syntax region LyraLilySchemeParen transparent matchgroup=LyraLilyDelimiter start=/(/ end=/)/ contained contains=@LyraLilySchemeItems
  syntax region LyraLilyScheme matchgroup=LyraLilyDelimiter start=/[#$]['`]\?(/ end=/)/ contained contains=@LyraLilySchemeItems
  syntax region LyraLilyEmbeddedMusic matchgroup=LyraLilyDelimiter start=/#{/ end=/#}/ contained contains=@LyraLilyScore

  syntax region LyraLilyDocument transparent start=/\%^/ end=/\%$/ contains=@LyraLilyScore keepend
  syntax sync fromstart
  let b:lyra_lilypond_syntax = 1
endfunction

call s:Palette()
augroup LyraLilyPondColors
  autocmd!
  autocmd BufReadPost,BufNewFile,BufWinEnter *.ly,*.ily call s:Apply(0)
  autocmd FileType lilypond call s:Apply(1)
  autocmd Syntax lilypond call s:Apply(1)
  autocmd ColorScheme * call s:Palette()
augroup END
call s:Apply(0)

let &cpo = s:cpo_save
unlet s:cpo_save
