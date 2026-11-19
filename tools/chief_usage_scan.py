import json, glob, os, sys, collections
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
        if o.get('type')=='user':
            c=o.get('message',{}).get('content')
            s=c if isinstance(c,str) else json.dumps(c,ensure_ascii=False)
            if '<command-name>/chief</command-name>' in s and not o.get('isSidechain'):
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
