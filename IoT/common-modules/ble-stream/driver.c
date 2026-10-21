#define _POSIX_C_SOURCE 200809L
#include "stream.h"
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <inttypes.h>
static bs_stream s;
static int unhex(const char*x,uint8_t*out,size_t cap){size_t n=strlen(x);if(n%2||n/2>cap)return -1;for(size_t i=0;i<n/2;i++){unsigned v;if(sscanf(x+2*i,"%2x",&v)!=1)return -1;out[i]=(uint8_t)v;}return (int)(n/2);}
int main(void){char*line=NULL;size_t capacity=0;uint8_t raw[8192],out[8192],gen[8];while(getline(&line,&capacity,stdin)>0){char op[8]={0},hex[16385]={0};unsigned cap=0,win=0,tok=0;int role=0,error=0;uint64_t now=0;size_t n=0;int result=0;sscanf(line,"%7s",op);
 if(!strcmp(op,"INIT")){sscanf(line,"%*s %d %16s %u %u %"SCNu64,&role,hex,&cap,&win,&now);if(unhex(hex,gen,8)!=8)return 3;bs_init(&s,role,gen,(uint16_t)cap,(uint16_t)win,now);}
 else if(!strcmp(op,"Q")){}
 else if(!strcmp(op,"I"))n=bs_info(&s,out);
 else if(!strcmp(op,"F")){sscanf(line,"%*s %16384s %"SCNu64,hex,&now);int len=unhex(hex,raw,sizeof raw);if(len<0)return 3;result=bs_feed(&s,raw,(size_t)len,now);}
 else if(!strcmp(op,"W")){sscanf(line,"%*s %16384s %"SCNu64,hex,&now);int len=unhex(hex,raw,sizeof raw);if(len<0)return 3;result=(int)bs_write(&s,raw,(size_t)len,now);}
 else if(!strcmp(op,"R")){sscanf(line,"%*s %u %"SCNu64,&cap,&now);if(cap>sizeof out)return 3;n=bs_read(&s,out,cap,now);}
 else if(!strcmp(op,"N")){sscanf(line,"%*s %"SCNu64,&now);n=bs_next(&s,out,now);}
 else if(!strcmp(op,"S")){sscanf(line,"%*s %16s %u %d %"SCNu64,hex,&tok,&error,&now);if(unhex(hex,gen,8)!=8)return 3;bs_settle(&s,gen,tok,error,now);}
 else if(!strcmp(op,"T")){sscanf(line,"%*s %"SCNu64,&now);bs_tick(&s,now);}
 else if(!strcmp(op,"X")){sscanf(line,"%*s %u %"SCNu64,&cap,&now);bs_close(&s,(uint16_t)cap,now);}
 else if(!strcmp(op,"O")){sscanf(line,"%*s %u",&cap);s.acked=s.dispatched=s.end=cap;s.read=s.next=cap;s.limit=(uint64_t)cap+s.capacity>BS_MAX?BS_MAX:cap+s.capacity;s.peer_limit=s.limit;}
 else if(!strcmp(op,"LIMIT")){/* Test driver only: reach production arithmetic boundaries without4GiB traffic. */s.acked=s.dispatched=s.end=BS_MAX-2;s.read=s.next=BS_MAX-2;s.limit=BS_MAX;s.peer_limit=BS_MAX;}
 else return 3;
 printf("{\"data\":\"");for(size_t i=0;i<n;i++)printf("%02x",out[i]);printf("\",\"result\":%d,\"phase\":%u,\"error\":%d,\"token\":%"PRIu32",\"flight\":%u,\"acked\":%"PRIu32",\"dispatched\":%"PRIu32",\"end\":%"PRIu32",\"read\":%"PRIu32",\"next\":%"PRIu32",\"limit\":%"PRIu32",\"peerLimit\":%"PRIu32",\"size\":%zu}\n",result,s.phase,s.error,s.local_token,s.nflight,s.acked,s.dispatched,s.end,s.read,s.next,s.limit,s.peer_limit,sizeof s);fflush(stdout);
 }free(line);return 0;}
