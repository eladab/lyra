" This file belongs only to the current Lyra child; never source it from vimrc.
let s:source = $LYRA_SOURCE_FILE
let s:draft = $LYRA_DRAFT ==# 'true'
let s:draft_buffer = bufnr(s:source)
let s:writing = 0
let s:pending_source = 0
" Only writers registered before Lyra can have completed the outer event when
" FirstWrite starts. A clean/unmodified draft alone is never proof of a write.
let s:earlier_writers = exists('*autocmd_get') ? autocmd_get({'event': 'BufWriteCmd'}) : []

function! s:HasEarlierWriter(path) abort
  for l:writer in s:earlier_writers
    if has_key(l:writer, 'bufnr')
      if l:writer.bufnr == s:draft_buffer
        return 1
      endif
    else
      let l:candidate = stridx(l:writer.pattern, '/') >= 0 ? a:path : fnamemodify(a:path, ':t')
      if l:candidate =~# glob2regpat(l:writer.pattern)
        return 1
      endif
    endif
  endfor
  return 0
endfunction

function! s:PublishSource() abort
  let l:marker = $LYRA_SOURCE_NOTIFICATION
  if writefile([json_encode(s:source)], l:marker . '.tmp') != 0 || rename(l:marker . '.tmp', l:marker) != 0
    throw 'Lyra: score saved, but source notification could not be published; retry :write'
  endif
  let s:pending_source = 0
endfunction

function! s:Notify(path) abort
  if !s:writing && resolve(a:path) ==# resolve(s:source)
    if s:pending_source
      call s:PublishSource()
    endif
    if !empty($LYRA_SAVE_FILE)
      silent! call writefile([reltimestr(reltime())], $LYRA_SAVE_FILE)
    endif
  endif
endfunction

function! s:InstallDraftWrite() abort
  " This draft has a custom first-write contract. acwrite also permits a retry
  " after a failed :saveas restored its original name; ordinary file buffers
  " otherwise retain Vim's 'not edited' flag and reject :write before our hook.
  call setbufvar(s:draft_buffer, '&buftype', 'acwrite')
  execute 'autocmd LyraDraftSave BufWriteCmd <buffer=' . s:draft_buffer . '> ++nested call <SID>FirstWrite(expand("<afile>:p"), v:cmdbang, v:cmdarg)'
endfunction

function! s:FirstWrite(target, force, options) abort
  let l:original = s:source
  let l:target = a:target
  let l:prompted = resolve(l:target) ==# resolve(l:original)
  if l:prompted
    call inputsave()
    try
      let l:answer = input('Save score as: ', fnamemodify(l:original, ':t'), 'file')
    catch /^Vim:Interrupt$/
      let l:answer = ''
    finally
      call inputrestore()
    endtry
    if empty(l:answer)
      setlocal modified
      throw 'Lyra: save cancelled'
    endif
    " fnamemodify expands a leading ~, while preserving literal spaces,
    " wildcard characters, quotes and Unicode instead of evaluating Ex text.
    let l:target = fnamemodify(l:answer, ':p')
  endif
  let l:force = a:force && !l:prompted
  " A user's earlier BufWriteCmd may already have completed an explicit write.
  " Trust its normal success contract (cleared 'modified') rather than mistaking
  " its newly created file for an unrelated existing destination. :saveas below
  " still lets Vim update file metadata and the user's custom writer own I/O.
  let l:already_written = !l:prompted && s:HasEarlierWriter(l:target) && !&modified && filereadable(l:target)
  let l:saved = 0
  try
    for l:reserved in [$LYRA_SOURCE_NOTIFICATION, empty($LYRA_SOURCE_NOTIFICATION) ? '' : $LYRA_SOURCE_NOTIFICATION . '.tmp', $LYRA_SAVE_FILE]
      if !empty(l:reserved) && resolve(l:target) ==# resolve(fnamemodify(l:reserved, ':p'))
        throw 'Lyra: choose a filename outside the internal session files'
      endif
    endfor
    if !l:already_written && !l:force && resolve(l:target) !=# resolve(l:original) && !empty(getftype(l:target))
      throw 'Lyra: file already exists; choose another name or use :saveas!'
    endif
    " Removing only our buffer-local handler lets genuine Vim perform the
    " write, including ordinary BufWritePre/Post hooks and file encodings.
    execute 'autocmd! LyraDraftSave BufWriteCmd <buffer=' . s:draft_buffer . '>'
    let s:writing = 1
    setlocal buftype=
    execute 'keepalt saveas' . ((l:force || l:already_written) ? '!' : '') . ' ' . a:options . ' ' . fnameescape(l:target)
    let l:saved = 1
    let s:source = expand('%:p')
    let s:draft = 0
    let s:pending_source = 1
    call s:PublishSource()
  finally
    let s:writing = 0
    if !l:saved
      setlocal modified
      " :saveas changes the buffer name before attempting disk I/O. Restore it
      " after an error so the next ordinary write asks again and cannot silently
      " overwrite the failed destination. A cancelled :wq must also stay open.
      if expand('%:p') !=# l:original
        execute 'keepalt file ' . fnameescape(l:original)
      endif
      call s:InstallDraftWrite()
    endif
  endtry
  call s:Notify(s:source)
endfunction

augroup LyraSessionSave
  autocmd!
  autocmd BufWritePost * call <SID>Notify(expand('<afile>:p'))
augroup END
if s:draft
  augroup LyraDraftSave
    autocmd!
  augroup END
  call s:InstallDraftWrite()
endif
