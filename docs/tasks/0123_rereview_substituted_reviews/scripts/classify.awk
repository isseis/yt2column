# Reads a commits file (records separated by "@@@COMMIT <sha>") and prints
# one signal line per matching record line:  <KIND> <sha12> <line>
function flush(   i) {
  for (i = 1; i <= nl; i++) {
    l = lines[i]
    if (l ~ /^(High review|Opus review): done / && !opus) print "OPUS-DONE-NONCLAUDE", sha, l
    else if (l ~ /^(High review|Opus review): done /) print "opus-done-claude", sha, l
    if (l ~ /^Implementation model: / && l !~ /claude-opus-/) print "IMPL-NONOPUS", sha, l
    if (l ~ /^Fable review: run / && l !~ /\(model claude-fable-/) print "FABLE-SUBST", sha, l
    else if (l ~ /^Fable review: run /) print "fable-ok", sha, l
    if (l ~ /^Fable review: not run /) print "FABLE-NOTRUN", sha, l
    if (l ~ /^Merge check: run / && l !~ /\(model claude-opus-/) print "MERGECHECK-SUBST", sha, l
    if (l ~ /^Merge check: not run /) print "MERGECHECK-NOTRUN", sha, l
    if (tolower(l) ~ /neon|argon|hydrogen|(opus|fable)[a-z ]* (was |is )?unavailable|instead of (claude )?(opus|fable)|in place of (claude )?(opus|fable)/ &&
        l !~ /^(Fable review: |Merge check: |High review: done |Opus review: done |Implementation model: )/) print "FREETEXT", sha, l
  }
  nl = 0; opus = 0
}
/^@@@COMMIT / { if (sha != "") flush(); sha = substr($2, 1, 12); next }
{ lines[++nl] = $0; if ($0 ~ /^Co-Authored-By: Claude Opus/) opus = 1 }
END { if (sha != "") flush() }
