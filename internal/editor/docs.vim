" Session-local integration. K remains a real Vim command, with the user's
" mappings and modes intact. Explicit :LyraDocs also works with a custom K.
if exists(':LyraDocs') == 2
  finish
endif
let s:notification = $LYRA_DOCS_FILE
let s:key = $LYRA_DOCS_KEY ==# 'true'

function! s:Lookup(query) abort
  let l:query = empty(a:query) ? expand('<cword>') : a:query
  let l:query = strpart(l:query, 0, 512)
  let l:tmp = s:notification . '.tmp'
  if writefile([json_encode([reltimestr(reltime()), l:query])], l:tmp) != 0 || rename(l:tmp, s:notification) != 0
    echoerr 'Lyra: documentation lookup could not be sent'
  endif
endfunction
command -nargs=* LyraDocs call <SID>Lookup(<q-args>)

function! s:Attach() abort
  if !s:key || expand('%:e') !~# '^i\?ly$'
    return
  endif
  " Preserve a plugin/user keyword program. The default is man (or man -s).
  if empty(&l:keywordprg) && index(['man', 'man -s'], &g:keywordprg) >= 0
    setlocal keywordprg=:LyraDocs
  endif
endfunction
augroup LyraDocumentation
  autocmd!
  autocmd BufEnter *.ly,*.ily call <SID>Attach()
  autocmd FileType * call <SID>Attach()
augroup END
call s:Attach()
