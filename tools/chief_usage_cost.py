import json,sys,collections
P={'claude-sonnet-5-5':(2,10,2.5,4,0.10),'claude-opus-5-5':(4,20,5,8,0.20)}
rows=json.load(open(sys.argv[1]))
def cost(m,v):
    p=P[m]; return (v.get('in',0)*p[0]+v.get('out',0)*p[1]+v.get('cw5',0)*p[2]+v.get('cw1h',0)*p[3]+v.get('cr',0)*p[4])/1e6
day=collections.defaultdict(lambda:[0,0,0]); tot=collections.Counter(); tok=collections.defaultdict(collections.Counter)
for first,last,f,u in rows:
    for m,v in u.items():
        if m not in P: continue
        c=cost(m,v); tot[m]+=c; tok[m].update(v)
        d=last[:10]; day[d][0 if m.endswith('sonnet-5-5') else 1]+=c; day[d][2]+=v['n']
for d in sorted(day): print(d, 'sonnet $%.2f'%day[d][0], 'opus $%.2f'%day[d][1])
for m in tot:
    t=tok[m]; print(m,'$%.2f'%tot[m],{k:t[k] for k in ('n','in','out','cr','cw5','cw1h')})
    p=P[m]; print('  breakdown: out $%.2f cr $%.2f cw5 $%.2f cw1h $%.2f in $%.2f'%(t['out']*p[1]/1e6,t['cr']*p[4]/1e6,t['cw5']*p[2]/1e6,t['cw1h']*p[3]/1e6,t['in']*p[0]/1e6))
# sonnet sessions detail
print('--- sonnet sessions')
for first,last,f,u in rows:
    v=u.get('claude-sonnet-5-5')
    if v:
        import datetime
        h=(datetime.datetime.fromisoformat(last[:19])-datetime.datetime.fromisoformat(first[:19])).total_seconds()/3600
        print(first[:16],f.split('/')[-1][:8],'%.1fh'%h,'calls',v['n'],'$%.2f'%cost('claude-sonnet-5-5',v))
