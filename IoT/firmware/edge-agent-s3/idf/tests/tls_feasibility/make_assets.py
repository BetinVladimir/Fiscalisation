import json,pathlib
p=pathlib.Path(__file__).parent
with (p/'assets.h').open('w') as f:
 for name,file in [('ca_pem','ca.pem'),('server_pem','server.pem'),('key_pem','server-key.pem'),('client_only_pem','client-only.pem'),('expired_pem','expired.pem')]: f.write('static const char '+name+'[]='+json.dumps((p/file).read_text())+';\n')
