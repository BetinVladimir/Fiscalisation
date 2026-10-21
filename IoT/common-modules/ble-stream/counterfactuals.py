from pathlib import Path
import subprocess,os,json
root=Path(__file__).resolve().parent
source=(root/'stream.c').read_text();header=(root/'stream.h').read_text();cases=[
 ('duplicate-suppression','duplicate suppression', 'if(end<=s->next){ack(s,now,1);return 0;}',''),
 ('future-ACK-rejection','queued/unsent', 'uint32_t lim=u32(f+16);','uint32_t lim=u32(f+16);if(off>s->dispatched)return 0;'),
 ('generation-fence','16 descriptor', '&&memcmp(f+4,s->gen,8))return 0;','&&0)return 0;'),
 ('SDK-deadline','16 descriptor','if(s->local&&now-s->local_at>=5000){fail(s,-7);return;}',''),
 ('TX-ring-bound','TX ring cap','size_t free=BS_CAP-(s->end-s->acked);','size_t free=BS_CAP;'),
 ('descriptor-bound','16 descriptor',None,None),
]
results=[]
for name,select,before,after in cases:
 folder=root/'counterfactuals'/name;folder.mkdir(parents=True,exist_ok=True)
 if before:assert source.count(before)==1,(name,source.count(before))
 (folder/'stream.c').write_text(source.replace(before,after) if before else source)
 (folder/'stream.h').write_text(header.replace('#define BS_FLIGHT 16u','#define BS_FLIGHT 17u') if not before else header)
 (folder/'driver.c').write_text((root/'driver.c').read_text())
 binary=folder/'peer';subprocess.run(['cc','-std=c11','-Wall','-Wextra','-Werror','-fsanitize=address,undefined','-g','stream.c','driver.c','-o',str(binary)],cwd=folder,check=True)
 env=os.environ|{'C_BINARY':str(binary),'TEST_FILTER':select}
 run=subprocess.run(['node','--experimental-strip-types','test.mjs'],cwd=root,env=env,stdout=subprocess.PIPE,stderr=subprocess.STDOUT,timeout=30)
 (folder/'red.log').write_bytes(run.stdout)
 assert run.returncode!=0,(name,'false green')
 assert b'AssertionError' in run.stdout,(name,'non-assert failure')
 results.append({'mutation':name,'expected':'RED assertion','exit':run.returncode,'test_filter':select})
 print('PASS expected RED',name,run.returncode)
(root/'counterfactual-results.json').write_text(json.dumps(results,indent=2)+'\n')
