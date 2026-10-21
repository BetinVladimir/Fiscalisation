/* Isolated feasibility fixture, not a production channel. */
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include "mbedtls/ssl.h"
#include "mbedtls/ctr_drbg.h"
#include "mbedtls/entropy.h"
#include "mbedtls/x509_crt.h"
#include "mbedtls/pk.h"
#include "psa/crypto.h"
#define CAP 8192
struct endpoint { mbedtls_ssl_context ssl; mbedtls_ssl_config cfg; mbedtls_x509_crt cert; mbedtls_pk_context key; mbedtls_entropy_context entropy; mbedtls_ctr_drbg_context rng; unsigned char in[CAP],out[CAP]; size_t ni,no; int ready; };
static int send_cb(void *p,const unsigned char*b,size_t n){struct endpoint*e=p;if(n>CAP-e->no)return MBEDTLS_ERR_SSL_WANT_WRITE;memcpy(e->out+e->no,b,n);e->no+=n;return (int)n;}
static int recv_cb(void*p,unsigned char*b,size_t n){struct endpoint*e=p;if(!e->ni)return MBEDTLS_ERR_SSL_WANT_READ;if(n>e->ni)n=e->ni;/* exercise 17-byte fragments */if(n>17)n=17;memcpy(b,e->in,n);e->ni-=n;memmove(e->in,e->in+n,e->ni);return (int)n;}
void channel_free(struct endpoint*e);
struct endpoint* channel_new(int server,const char*cert,const char*key,const char*name){int r;struct endpoint*e=calloc(1,sizeof(*e));if(!e)return NULL;mbedtls_ssl_init(&e->ssl);mbedtls_ssl_config_init(&e->cfg);mbedtls_x509_crt_init(&e->cert);mbedtls_pk_init(&e->key);mbedtls_entropy_init(&e->entropy);mbedtls_ctr_drbg_init(&e->rng);
#define OK(x) do{r=(x);if(r){fprintf(stderr,"init %s: -%x\n",#x,-r);goto fail;}}while(0)
OK(psa_crypto_init());OK(mbedtls_ctr_drbg_seed(&e->rng,mbedtls_entropy_func,&e->entropy,(const unsigned char*)"isolated TLS spike",18));OK(mbedtls_ssl_config_defaults(&e->cfg,server?MBEDTLS_SSL_IS_SERVER:MBEDTLS_SSL_IS_CLIENT,MBEDTLS_SSL_TRANSPORT_STREAM,MBEDTLS_SSL_PRESET_DEFAULT));mbedtls_ssl_conf_rng(&e->cfg,mbedtls_ctr_drbg_random,&e->rng);mbedtls_ssl_conf_min_tls_version(&e->cfg,MBEDTLS_SSL_VERSION_TLS1_3);mbedtls_ssl_conf_max_tls_version(&e->cfg,MBEDTLS_SSL_VERSION_TLS1_3);
#if defined(MBEDTLS_SSL_SESSION_TICKETS) && defined(MBEDTLS_SSL_SRV_C)
mbedtls_ssl_conf_new_session_tickets(&e->cfg,0);
#endif
OK(mbedtls_x509_crt_parse(&e->cert,(const unsigned char*)cert,strlen(cert)+1));
if(server){OK(mbedtls_pk_parse_key(&e->key,(const unsigned char*)key,strlen(key)+1,NULL,0,mbedtls_ctr_drbg_random,&e->rng));OK(mbedtls_ssl_conf_own_cert(&e->cfg,&e->cert,&e->key));mbedtls_ssl_conf_authmode(&e->cfg,MBEDTLS_SSL_VERIFY_NONE);}else{mbedtls_ssl_conf_ca_chain(&e->cfg,&e->cert,NULL);mbedtls_ssl_conf_authmode(&e->cfg,MBEDTLS_SSL_VERIFY_REQUIRED);}
OK(mbedtls_ssl_setup(&e->ssl,&e->cfg));if(!server){OK(mbedtls_ssl_set_hostname(&e->ssl,name));}mbedtls_ssl_set_bio(&e->ssl,e,send_cb,recv_cb,NULL);return e;
fail:channel_free(e);return NULL;
}
int channel_feed(struct endpoint*e,const unsigned char*p,int n){if(n<0||(size_t)n>CAP-e->ni)return -1;memcpy(e->in+e->ni,p,n);e->ni+=n;return n;}
int channel_drain(struct endpoint*e,unsigned char*p,int n){if(n<0)return -1;if((size_t)n>e->no)n=e->no;memcpy(p,e->out,n);e->no-=n;memmove(e->out,e->out+n,e->no);return n;}
int channel_step(struct endpoint*e){int r=mbedtls_ssl_handshake(&e->ssl);if(r==0){e->ready=1;return 1;}if(r==MBEDTLS_ERR_SSL_WANT_READ||r==MBEDTLS_ERR_SSL_WANT_WRITE)return 0;return r;}
int channel_write(struct endpoint*e,const unsigned char*p,int n){if(!e->ready)return -1;return mbedtls_ssl_write(&e->ssl,p,n);}
int channel_read(struct endpoint*e,unsigned char*p,int n){if(!e->ready)return -1;return mbedtls_ssl_read(&e->ssl,p,n);}
void channel_free(struct endpoint*e){if(!e)return;mbedtls_ssl_free(&e->ssl);mbedtls_ssl_config_free(&e->cfg);mbedtls_x509_crt_free(&e->cert);mbedtls_pk_free(&e->key);mbedtls_ctr_drbg_free(&e->rng);mbedtls_entropy_free(&e->entropy);memset(e,0,sizeof(*e));free(e);}
