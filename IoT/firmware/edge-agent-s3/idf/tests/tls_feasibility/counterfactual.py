import pathlib,subprocess,shutil
p=pathlib.Path(__file__).parent;d=p/'counterfactual';d.mkdir(exist_ok=True)
for f in ['main.c','assets.h']:shutil.copy2(p/f,d/f)
s=(p/'engine.c').read_text().replace('mbedtls_ssl_conf_authmode(&e->cfg,MBEDTLS_SSL_VERIFY_REQUIRED)','mbedtls_ssl_conf_authmode(&e->cfg,MBEDTLS_SSL_VERIFY_NONE)').replace('mbedtls_ssl_set_hostname(&e->ssl,name)','mbedtls_ssl_set_hostname(&e->ssl,"adapter.spike.test")')
assert s!=(p/'engine.c').read_text();(d/'engine.c').write_text(s)
subprocess.run(['cc','-O2','-I',str(p/'mbedtls-3.6.7/include'),str(d/'main.c'),'-o',str(d/'fixture'),*[str(p/'build-host/library'/f) for f in ['libmbedtls.a','libmbedx509.a','libmbedcrypto.a']]],check=True)
for mode in ['wrong.spike.test','expired']:
 r=subprocess.run([str(d/'fixture'),mode],capture_output=True,text=True,timeout=10)
 print(r.stdout+r.stderr,end='');print('COUNTERFACTUAL',mode,'exit=',r.returncode)
 assert r.returncode==15,'Negative test must RED on forbidden successful handshake'
print('PASS counterfactual guards detect disabled hostname/client verification')

# Force a ClientAuth-only leaf from an independent server to the actual C client.
for f in ['peer_client.c','peer_server.go','ca.pem','server.pem','server-key.pem','client-only.pem']:shutil.copy2(p/f,d/f)
subprocess.run(['cc','-O2','-I',str(p/'mbedtls-3.6.7/include'),str(d/'peer_client.c'),'-o',str(d/'c-peer-client'),*[str(p/'build-host/library'/f) for f in ['libmbedtls.a','libmbedx509.a','libmbedcrypto.a']]],check=True)
r=subprocess.run(['go','run','peer_server.go'],cwd=d,capture_output=True,text=True,timeout=30)
print(r.stdout+r.stderr,end='')
assert r.returncode!=0 and 'forbidden EKU handshake accepted' in r.stderr,'Independent-peer EKU test must RED when client verification is disabled'
print('PASS independent-peer client-EKU counterfactual detects disabled verification')
