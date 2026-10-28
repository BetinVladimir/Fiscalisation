#include "stream.h"
#include <assert.h>
#include <stdio.h>
#include <string.h>
static const uint8_t gen[8]={1,2,3,4,5,6,7,8};
static void opened(bs_stream*s){uint8_t frame[20]={0x42,0x53,1,1,1,2,3,4,5,6,7,8,0,0,0,0,0,20,8,0},out[BS_FRAME];bs_init(s,1,gen,20,2048,0);assert(bs_feed(s,frame,20,0)==0);assert(bs_next(s,out,0)==20);bs_settle(s,gen,s->local_token,0,0);assert(s->phase==BS_OPEN);}
static void wiped(const bs_stream*s){assert(s->phase==BS_CLOSED);for(size_t i=0;i<BS_CAP;i++)assert(s->tx[i]==0&&s->rx[i]==0);}
int main(void){bs_stream s;uint8_t one=0xa5,out=0;opened(&s);assert(bs_write(&s,&one,1,0)==1);assert(s.end==1);assert(bs_write(&s,&one,SIZE_MAX,1)==0);assert(s.error==-5);wiped(&s);
 opened(&s);assert(bs_write(&s,&one,SIZE_MAX,0)==0);wiped(&s);
 opened(&s);assert(bs_write(&s,&one,1,0)==1);assert(bs_write(&s,&one,(size_t)BS_MAX,1)==0);wiped(&s);
 opened(&s);assert(bs_write(&s,&one,0,0)==0&&s.phase==BS_OPEN);
 opened(&s);assert(bs_feed(&s,&one,SIZE_MAX,1)<0);wiped(&s);
 opened(&s);assert(bs_feed(&s,NULL,SIZE_MAX,1)<0);wiped(&s);
 opened(&s);uint8_t f[17]={0x42,0x53,1,3,1,2,3,4,5,6,7,8,0,0,0,0,0xa5};assert(bs_feed(&s,f,17,0)==0);assert(bs_read(&s,&out,SIZE_MAX,1)==1&&out==0xa5);assert(s.read==1);
 puts("PASS independent C SIZE_MAX write/feed/read + zero count; no allocation");return 0;}
