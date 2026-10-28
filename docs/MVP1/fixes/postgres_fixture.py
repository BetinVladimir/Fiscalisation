"""Disposable PostgreSQL for schema acceptance; never connects to existing servers."""
from contextlib import contextmanager
from pathlib import Path
import os,shutil,subprocess,tempfile,urllib.parse

@contextmanager
def temporary_postgres():
    tools={name:shutil.which(name) for name in ('initdb','pg_ctl','psql')}
    if not all(tools.values()):raise RuntimeError('BLOCKED: initdb/pg_ctl/psql required')
    # Unix socket paths have a strict length limit; artifact paths can be too long.
    with tempfile.TemporaryDirectory(prefix='mvp1-pg-',dir='/private/tmp') as raw:
        base=Path(raw);data=base/'data';sock=base/'sock';sock.mkdir(mode=0o700)
        log=base/'postgres.log';started=False
        def run(argv,timeout=60):
            p=subprocess.run(argv,capture_output=True,text=True,timeout=timeout)
            if p.returncode:raise RuntimeError('PostgreSQL setup failed: '+p.stderr[-3000:]+p.stdout[-1000:])
            return p
        try:
            run([tools['initdb'],'-D',str(data),'-U','postgres','-A','trust','--no-locale','--encoding=UTF8','--no-sync'])
            run([tools['pg_ctl'],'-D',str(data),'-l',str(log),'-o',"-c listen_addresses='' -k "+str(sock)+' -p 55432','-w','-t','30','start'])
            started=True
            url='postgresql://postgres@/postgres?'+urllib.parse.urlencode({'host':str(sock),'port':'55432'})
            probe=run([tools['psql'],url,'-X','-A','-t','-v','ON_ERROR_STOP=1','-c','select 1'])
            if probe.stdout.strip()!='1':raise RuntimeError('PostgreSQL readiness probe failed')
            yield url
        finally:
            # A failed start can still leave the process running: stop only OUR datadir.
            if started or (data/'postmaster.pid').exists():
                stopped=subprocess.run([tools['pg_ctl'],'-D',str(data),'-m','immediate','-w','-t','30','stop'],capture_output=True,text=True,timeout=40)
                if stopped.returncode:raise RuntimeError('Owned PostgreSQL cleanup failed: '+stopped.stderr[-1000:])
            artifact=os.environ.get('MVP1_ARTIFACT_DIR')
            if artifact and log.exists():
                out=Path(artifact);out.mkdir(parents=True,exist_ok=True);shutil.copy2(log,out/(base.name+'.postgres.log'))
