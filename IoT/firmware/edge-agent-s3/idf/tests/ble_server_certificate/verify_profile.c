/* Host verifier of Go-produced, synthetic BLE leaf fixtures. No device key IO. */
#include <stdio.h>
#include <string.h>
#include "mbedtls/x509_crt.h"
#include "mbedtls/oid.h"
#include "mbedtls/sha256.h"
#include "mbedtls/pk.h"
int main(int argc,char**argv){
 if(argc!=5)return 20;
 mbedtls_x509_crt leaf,ca;mbedtls_x509_crt_init(&leaf);mbedtls_x509_crt_init(&ca);
 int r=mbedtls_x509_crt_parse_file(&leaf,argv[1]);if(!r)r=mbedtls_x509_crt_parse_file(&ca,argv[2]);
 uint32_t flags=0;if(!r)r=mbedtls_x509_crt_verify(&leaf,&ca,NULL,argv[3],&flags,NULL,NULL);
 if(!r)r=mbedtls_x509_crt_check_extended_key_usage(&leaf,MBEDTLS_OID_SERVER_AUTH,MBEDTLS_OID_SIZE(MBEDTLS_OID_SERVER_AUTH));
 /* Exact enrolled SPKI pin is additional to normal CA/name/EKU validation. */
 unsigned char der[512],hash[32];char hex[65];
 if(!r){int n=mbedtls_pk_write_pubkey_der(&leaf.pk,der,sizeof der);if(n<1)r=-1;else if(mbedtls_sha256(der+sizeof(der)-n,(size_t)n,hash,0))r=-2;else{for(int i=0;i<32;i++)sprintf(hex+2*i,"%02x",hash[i]);hex[64]=0;if(strcmp(hex,argv[4]))r=-3;}}
 mbedtls_x509_crt_free(&leaf);mbedtls_x509_crt_free(&ca);
 if(r){fprintf(stderr,"REJECT profile r=%d flags=%u\n",r,(unsigned)flags);return 1;}
 puts("PASS actual mbedTLS CA/name/ServerAuth/enrolled-SPKI verification");return 0;
}
