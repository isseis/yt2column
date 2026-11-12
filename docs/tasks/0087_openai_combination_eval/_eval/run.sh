#!/bin/zsh
# Task 0087's run script (design section 3.1-section 3.3). It holds the config table, the
# only place the configs' LLM values are written, and runs the pinned CLI for
# one article of one config.
#
# Usage:
#   run.sh configs
#       Print the config table: "<id>\t<stage>\t<YT2COLUMN_LLM>\t<YT2COLUMN_REVIEW_LLM>\t<mode>".
#   run.sh run <config> <article> <cli> [--eval-root <dir>] [--launch-token <token> | --preflight [--after-review]]
#       Run <cli> for <article> ("<video ID>-<1|2>") with <config>'s values.
#       --preflight writes under 0087/preflight/ instead of 0087/ and removes
#       the API keys from the CLI's environment, so the CLI cannot call an API;
#       --after-review (preflight only) resumes from a copy of B3's complete
#       artifacts with --after review, whatever the config (design section 3.4 item 1),
#       and writes under 0087/preflight/after-review/ instead, a root of its own,
#       so these runs and the config checks of item 2 share no output path.
#
# A run without --preflight calls a paid API, so it starts only from
# "evaltool launch": launch holds 0087/launch.lock with a per-launch token and
# hands the same token with --launch-token; without a match the script stops
# before copying anything.
#
# The evaluation root is --eval-root (launch passes the root it checked), else
# $YT2COLUMN_EVAL_ROOT, or ~/yt2column-eval when neither is given.
# The script never reads, writes, or prints an API key's value: the CLI reads
# the keys from the environment it inherits.
#
# It assumes macOS (shasum, BSD stat), the machine the evaluation runs on.

emulate -L zsh
set -u
setopt pipefail

# The config table. mode "full" runs every stage; "resume" reuses B3's
# generation (and, for roNZ-rLxhUE-1, its shorten) and runs the rest.
CONFIGS=(
  $'C-sol\t1\topenai/gpt-6.1-sol+medium\topenai/gpt-6.1-sol+medium\tfull'
  $'C-astra\t1\topenai/gpt-6-astra+medium\topenai/gpt-6-astra+medium\tfull'
  $'A-sol\t1\tclaude/claude-opus-5-5+high\topenai/gpt-6.1-sol+medium\tresume'
  $'A-astra\t1\tclaude/claude-opus-5-5+high\topenai/gpt-6-astra+medium\tresume'
  $'B-sol\t2\topenai/gpt-6.1-sol+medium\tclaude/claude-opus-5-5+high\tfull'
  $'B-astra\t2\topenai/gpt-6-astra+medium\tclaude/claude-opus-5-5+high\tfull'
)

# The five videos of the evaluation.
VIDEOS=(roNZ-rLxhUE 18tbwwPM91A zW1RBacSqjk leE02uqgaKg EqnGQDU71-Y)

# The article B3 resumed after shorten (its shorten ran in B3's retry).
RESUME_AFTER_SHORTEN=roNZ-rLxhUE-1

# CLEANUP holds the directories this run created before the CLI starts; a
# failure before the start removes them, so a retry is not blocked by them.
CLEANUP=()

die() {
  print -r -- "run.sh: $*" >&2
  (( ${#CLEANUP} )) && rm -rf -- $CLEANUP
  exit 1
}

# copy_checked <src> <dst> <hash-list key>: copy the file's content only, mode
# 0600, and check its SHA-256 against the first hash list's entry for the key.
copy_checked() {
  local src=$1 dst=$2 key=$3 want got
  [[ -f $src && ! -L $src ]] || die "not a regular file: $src"
  cat -- $src > $dst || die "cannot copy $src"
  chmod 600 $dst || die "cannot chmod $dst"
  want=$(awk -F'\t' -v k="$key" '$1 == k { n++; h = $3 } END { if (n == 1) print h }' $LIST) || die "cannot read $LIST"
  [[ -n $want ]] || die "the hash list has no single entry for $key"
  got=$(shasum -a 256 -- $dst | cut -d' ' -f1) || die "cannot hash $dst"
  [[ $got == $want ]] || die "the copy of $key differs from the hash list"
}

cmd_configs() {
  local row
  for row in $CONFIGS; do print -r -- $row; done
}

cmd_run() {
  (( $# >= 3 )) || die "usage: run.sh run <config> <article> <cli> [--preflight [--after-review]]"
  local cfg=$1 art=$2 cli=$3
  shift 3
  local preflight=0 after_review=0 token= eval_root=
  while (( $# )); do
    case $1 in
      --preflight) preflight=1 ;;
      --after-review) after_review=1 ;;
      --launch-token) (( $# >= 2 )) || die "--launch-token needs a value"; token=$2; shift ;;
      --eval-root) (( $# >= 2 )) || die "--eval-root needs a value"; eval_root=$2; shift ;;
      *) die "unknown option $1" ;;
    esac
    shift
  done
  (( after_review == 0 || preflight == 1 )) || die "--after-review needs --preflight"

  local row fields llm review mode
  row=${${(M)CONFIGS:#${cfg}$'\t'*}[1]-}
  [[ -n $row ]] || die "unknown config $cfg"
  fields=("${(@ps:\t:)row}")
  llm=$fields[3] review=$fields[4] mode=$fields[5]

  [[ $art =~ '^[A-Za-z0-9_-]{11}-[12]$' ]] || die "not an article: $art"
  local vid=${art%-*}
  (( ${VIDEOS[(Ie)$vid]} )) || die "not a video of the evaluation: $vid"
  # An absolute path: the file hashed below is then the file env runs, never
  # one found through PATH.
  [[ $cli == /* && -x $cli && -f $cli ]] || die "not an executable CLI given by an absolute path: $cli"

  local E=${eval_root:-${YT2COLUMN_EVAL_ROOT:-$HOME/yt2column-eval}}
  local R=$E/0087
  (( preflight )) && R=$E/0087/preflight
  (( after_review )) && R=$E/0087/preflight/after-review
  LIST=$E/0087/hash/1-initial.tsv
  [[ -f $LIST ]] || die "no first hash list $LIST"
  if (( ! preflight )); then
    local lock=$E/0087/launch.lock
    [[ -n $token && -f $lock && "$(cat -- $lock)" == "$token" ]] ||
      die "a run that calls an API starts only from evaltool launch (no matching launch token)"
  fi

  # Which artifacts to copy, and from which tree of the hash list.
  local after=none tree= src=
  local -a files
  if (( after_review )); then
    after=review tree=work src=$E/0018/work/B3/$art
    files=($src/*.json(N:t))
  elif [[ $mode == resume && $art == $RESUME_AFTER_SHORTEN ]]; then
    after=shorten tree=work src=$E/0018/work/B3/$art
    files=($vid.generation.json $vid.shorten.json)
  elif [[ $mode == resume ]]; then
    after=generation tree=base src=$E/0018/base/B3/$art
    files=($vid.generation.json)
  fi
  if [[ $after != none ]]; then
    [[ -d $src ]] || die "no artifacts to resume from: $src"
    (( ${#files} )) || die "no artifacts in $src"
  fi

  local W=$R/work/$cfg/$art O=$R/out/$cfg/$art.md V=$R/review/$cfg/$art.md
  local L=$R/log/$cfg/$art.txt WH=$R/log/$cfg/$art.when C=$R/runcache/$cfg/$art
  local p
  for p in $W $O $V $L $WH $C; do
    [[ -e $p || -L $p ]] && die "already exists: $p"
  done
  umask 077
  mkdir -p $R/work/$cfg $R/out/$cfg $R/review/$cfg $R/log/$cfg $R/runcache/$cfg || die "cannot create $R"
  # Before the CLI starts, a failure removes whatever of these this run made.
  CLEANUP=($W $C $WH)
  mkdir -m 700 $W $C || die "cannot create $W or $C"

  # The run's own cache copy: the video's pointer and the slot it names.
  local slot f
  slot=$(cat -- $E/cache/$vid.current) || die "no cache pointer for $vid"
  [[ $slot == a || $slot == b ]] || die "the cache pointer of $vid names no slot"
  copy_checked $E/cache/$vid.current $C/$vid.current cache/$vid.current
  mkdir -m 700 $C/$vid.$slot || die "cannot create $C/$vid.$slot"
  for f in $E/cache/$vid.$slot/*(N); do
    copy_checked $f $C/$vid.$slot/${f:t} cache/$vid.$slot/${f:t}
  done
  for f in $files; do
    copy_checked $src/$f $W/$f $tree/$art/$f
  done

  # The run conditions the record states and the CLI receives come from the
  # same two values.
  local ytdlp=/usr/bin/false keep=--keep-cache

  local sha
  sha=$(shasum -a 256 -- $cli | cut -d' ' -f1) || die "cannot hash $cli"
  {
    print -r -- "config=$cfg"
    print -r -- "article=$art"
    print -r -- "after=$after"
    print -r -- "llm=$llm"
    print -r -- "review_llm=$review"
    print -r -- "ytdlp_path=$ytdlp"
    print -r -- "keep_cache=${${keep:+yes}:-no}"
    print -r -- "binary_sha256=$sha"
    print -r -- "start=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  } > $WH || die "cannot write $WH"
  # From here on the run's files are kept, whatever happens.
  CLEANUP=()

  local -a args
  args=($keep --artifacts $W --out $O)
  if [[ $after == review ]]; then
    args+=(--after review)
  else
    args+=(--review-log $V)
    [[ $after != none ]] && args+=(--after $after)
  fi

  # Only the values set here reach the CLI from the YT2COLUMN_ family.
  local v
  for v in ${(k)parameters[(I)YT2COLUMN_*]}; do unset $v; done
  unset ANTHROPIC_WORKSPACE_ID
  (( preflight )) && unset ANTHROPIC_API_KEY OPENAI_API_KEY DEEPSEEK_API_KEY
  # zsh's default TIMEFMT, the form of the 0018 logs.
  TIMEFMT='%J  %U user %S system %P cpu %*E total'
  # The CLI runs in the background so a signal to this script (Ctrl-C, a
  # TERM, a hangup) reaches the trap: the trap asks the CLI to stop with
  # SIGTERM, which it answers by writing its usage and cost lines, and the
  # script still waits for it and closes the run record. Without this, an
  # interrupted run would leave a record without end and exit lines.
  local rc pid
  ( time env YT2COLUMN_YTDLP_PATH=$ytdlp YT2COLUMN_CACHE_DIR=$C \
      YT2COLUMN_LLM=$llm YT2COLUMN_REVIEW_LLM=$review \
      $cli $args "https://www.youtube.com/watch?v=$vid" ) 2> $L &
  pid=$!
  trap 'pkill -TERM -P $pid 2>/dev/null' INT TERM HUP
  wait $pid
  rc=$?
  # A trapped signal ends wait early; wait again until the CLI has exited.
  while kill -0 $pid 2>/dev/null; do
    wait $pid
    rc=$?
  done
  trap - INT TERM HUP
  {
    print -r -- "end=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    print -r -- "exit=$rc"
  } >> $WH || die "cannot finish $WH"
  return $rc
}

(( $# >= 1 )) || die "usage: run.sh configs | run.sh run <config> <article> <cli> [...]"
case $1 in
  configs) cmd_configs ;;
  run) shift; cmd_run "$@" ;;
  *) die "unknown command $1" ;;
esac
