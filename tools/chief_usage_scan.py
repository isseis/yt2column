import json, glob, os, re, sys, collections

# A slash command the human typed is recorded as a user record whose content is
# a string that starts with the command tags. The tag names are joined from
# parts so that this source, when a session reads it, does not itself look like
# a recorded command.
_TAG = 'command-' + 'name'
_COMMAND = re.compile(
    r'\A\s*(?:<command-message>[^<]*</command-message>\s*)?<' + _TAG + r'>([^<]*)</' + _TAG + '>')
_CHIEF = '/' + 'chief'


def typed_command(o):
    """Return the command name of a human-typed slash command record, else None.

    Tool results (list content), meta records, compact summaries, and sidechain
    records are never a human-typed command.
    """
    if o.get('type') != 'user' or o.get('isMeta') or o.get('isSidechain') or o.get('isCompactSummary'):
        return None
    m = o.get('message')
    c = m.get('content') if isinstance(m, dict) else None
    if not isinstance(c, str):
        return None
    hit = _COMMAND.match(c)
    return hit.group(1).strip() if hit else None

rows=[]
for f in glob.glob(os.path.expanduser('~/.claude/projects/*/*.jsonl')):
    try:
        lines=open(f,encoding='utf-8',errors='replace').read().splitlines()
    except Exception: continue
    chief=False; first=None
    seen=set(); usage=collections.defaultdict(lambda: collections.Counter()); ts0=ts1=None
    for ln in lines:
        try: o=json.loads(ln)
        except: continue
        t=o.get('timestamp')
        if typed_command(o)==_CHIEF:
            chief=True; first=first or t
        if o.get('type')=='assistant' and chief:
            m=o.get('message',{}); mid=m.get('id'); u=m.get('usage')
            if not u or mid in seen: continue
            seen.add(mid)
            k=usage[m.get('model')]
            k['in']+=u.get('input_tokens',0); k['out']+=u.get('output_tokens',0)
            k['cr']+=u.get('cache_read_input_tokens',0)
            cc=u.get('cache_creation') or {}
            k['cw5']+=cc.get('ephemeral_5m_input_tokens',0); k['cw1h']+=cc.get('ephemeral_1h_input_tokens',0)
            if not cc: k['cw5']+=u.get('cache_creation_input_tokens',0)
            k['n']+=1
            ts0=ts0 or t; ts1=t
    if chief:
        rows.append((first,ts1,f,{m:dict(v) for m,v in usage.items()}))
rows.sort()
json.dump(rows,open(sys.argv[1],'w'),indent=1)
for r in rows: print(r[0],r[1],os.path.basename(os.path.dirname(r[2]))[-30:],os.path.basename(r[2])[:8],{m:(v['n'],v['out']) for m,v in r[3].items()})
