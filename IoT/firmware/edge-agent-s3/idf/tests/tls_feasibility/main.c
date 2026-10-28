#include "engine.c"
#include "assets.h"
#include <unistd.h>
static int finish(struct endpoint*c,struct endpoint*s,int rc){channel_free(c);channel_free(s);return rc;}
int main(int argc,char**argv){
 const char*mode=argc>1?argv[1]:"positive";
 int wrong=!strcmp(mode,"wrong.spike.test"),expired=!strcmp(mode,"expired"),client_only=!strcmp(mode,"client-only");
 int negative=wrong||expired||client_only;
 if(argc>1&&!negative&&strcmp(mode,"server")){fprintf(stderr,"unknown mode\n");return 16;}
 struct endpoint*s=channel_new(1,client_only?client_only_pem:expired?expired_pem:server_pem,key_pem,NULL);
 unsigned char b[8192];if(!s)return 1;
 if(!strcmp(mode,"server")){
  for(;;){int r=channel_step(s),n=channel_drain(s,b,sizeof b);
   if(n&&write(1,b,n)!=n)return finish(NULL,s,2);
   if(r<0){fprintf(stderr,"handshake -%x\n",-r);return finish(NULL,s,3);}
   if(r==1){int k=channel_read(s,b,sizeof b);if(k>0){channel_write(s,b,k);n=channel_drain(s,b,sizeof b);int rc=write(1,b,n)==n?0:2;return finish(NULL,s,rc);}if(k!=MBEDTLS_ERR_SSL_WANT_READ)return finish(NULL,s,4);}
   n=read(0,b,sizeof b);if(n<=0)return finish(NULL,s,5);if(channel_feed(s,b,n)<0)return finish(NULL,s,6);
  }
 }
 struct endpoint*c=channel_new(0,ca_pem,NULL,wrong?mode:"adapter.spike.test");if(!c)return finish(NULL,s,7);
 for(int i=0;i<20000&&!(s->ready&&c->ready);i++){
  int r=channel_step(c);
  if(r<0){uint32_t flags=mbedtls_ssl_get_verify_result(&c->ssl);uint32_t expected=wrong?MBEDTLS_X509_BADCERT_CN_MISMATCH:expired?MBEDTLS_X509_BADCERT_EXPIRED:0;
   if(expected&&r==MBEDTLS_ERR_X509_CERT_VERIFY_FAILED&&flags==expected){printf("PASS expected client rejection mode=%s error=-%x verifyflags=%x\n",mode,-r,(unsigned)flags);return finish(c,s,0);}
   fprintf(stderr,"FAIL unexpected client rejection mode=%s error=-%x flags=%x expected=%x\n",mode,-r,(unsigned)flags,(unsigned)expected);return finish(c,s,8);
  }
  int n=channel_drain(c,b,31);if(channel_feed(s,b,n)<0)return finish(c,s,6);r=channel_step(s);
  if(r<0){if(client_only&&r==MBEDTLS_ERR_SSL_HANDSHAKE_FAILURE){printf("PASS expected server certificate-selection rejection mode=client-only error=-%x\n",-r);return finish(c,s,0);}fprintf(stderr,"FAIL unexpected server rejection -%x\n",-r);return finish(c,s,9);}
  n=channel_drain(s,b,23);if(channel_feed(c,b,n)<0)return finish(c,s,6);
 }
 if(!(s->ready&&c->ready))return finish(c,s,10);
 if(negative){fprintf(stderr,"FAIL forbidden handshake succeeded mode=%s\n",mode);return finish(c,s,15);}
 const char*msg="synthetic read-only payload";int n=channel_write(c,(const unsigned char*)msg,strlen(msg));if(n!=(int)strlen(msg))return finish(c,s,11);
 while((n=channel_drain(c,b,29)))if(channel_feed(s,b,n)<0)return finish(c,s,6);
 n=channel_read(s,b,sizeof b);if(n!=(int)strlen(msg)||memcmp(b,msg,n))return finish(c,s,12);
 printf("PASS memory BIO TLS1.3 fragmented C-to-C %s endpoint_bytes=%zu\n",mbedtls_ssl_get_ciphersuite(&c->ssl),sizeof(*c));return finish(c,s,0);
}
