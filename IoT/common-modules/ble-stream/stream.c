#include "stream.h"
#include <string.h>
_Static_assert(sizeof(bs_stream)<=5120,"stream static budget");
static uint16_t u16(const uint8_t*p){return (uint16_t)(((uint16_t)p[0]<<8)|p[1]);}
static uint32_t u32(const uint8_t*p){return ((uint32_t)p[0]<<24)|((uint32_t)p[1]<<16)|((uint32_t)p[2]<<8)|p[3];}
static void p16(uint8_t*p,uint16_t v){p[0]=(uint8_t)(v>>8);p[1]=(uint8_t)v;}
static void p32(uint8_t*p,uint32_t v){p[0]=(uint8_t)(v>>24);p[1]=(uint8_t)(v>>16);p[2]=(uint8_t)(v>>8);p[3]=(uint8_t)v;}
static int fail(bs_stream*s,int e){memset(s->tx,0,sizeof s->tx);memset(s->rx,0,sizeof s->rx);memset(s->flight,0,sizeof s->flight);s->nflight=0;s->local=0;s->phase=BS_CLOSED;s->error=e;s->pending_ack=s->pending_open=s->pending_openack=s->pending_close=s->pending_closeack=s->replay=0;return e;}
static int clock_ok(bs_stream*s,uint64_t now){if(now<s->last_tick||now>UINT64_MAX-120000){fail(s,-7);return 0;}s->last_tick=now;return 1;}
static size_t head(bs_stream*s,uint8_t*out,uint8_t type,uint32_t offset){out[0]=0x42;out[1]=0x53;out[2]=1;out[3]=type;memcpy(out+4,s->gen,8);p32(out+12,offset);return 16;}
static int validcap(uint16_t c){return c>=20&&c<=BS_FRAME;}
static int validwindow(uint16_t c){return c>=256&&c<=BS_CAP;}
void bs_init(bs_stream*s,int server,const uint8_t gen[8],uint16_t cap,uint16_t capacity,uint64_t now){memset(s,0,sizeof *s);s->server=server!=0;memcpy(s->gen,gen,8);s->cap=cap;s->capacity=capacity;s->start=now;s->last_tick=now;s->phase=BS_INIT;if(now>UINT64_MAX-120000){fail(s,-7);return;}if(!validcap(cap)||!validwindow(capacity)||(server&&!memcmp(gen,"\0\0\0\0\0\0\0\0",8)))fail(s,-1);}
size_t bs_info(bs_stream*s,uint8_t out[20]){if(!s->server||s->phase==BS_CLOSED)return 0;head(s,out,0,0);p16(out+16,s->cap);p16(out+18,s->capacity);return 20;}
static void ack(bs_stream*s,uint64_t now,int immediate){if(!s->pending_ack)s->ack_due=now+(immediate?0:25);if(immediate)s->ack_due=now;s->pending_ack=1;}
int bs_feed(bs_stream*s,const uint8_t*f,size_t n,uint64_t now){
 if(s->phase==BS_CLOSED)return -9;
 if(!f||n>BS_FRAME)return fail(s,-2);
 if(!clock_ok(s,now))return -7;
 if(n>=12&&!(s->phase==BS_INIT&&!s->server)&&memcmp(f+4,s->gen,8))return 0;
 if(n<16||n>BS_FRAME||f[0]!=0x42||f[1]!=0x53||f[2]!=1||f[3]>6)return fail(s,-2);
 uint8_t type=f[3];uint32_t off=u32(f+12);
 if(type!=3&&n!=(type>=5?18u:20u))return fail(s,-2);
 if(type!=3&&type!=4&&off)return fail(s,-2);
 if(type==0){if(s->server||s->phase!=BS_INIT||!memcmp(f+4,"\0\0\0\0\0\0\0\0",8)||!validcap(u16(f+16))||!validwindow(u16(f+18)))return fail(s,-2);memcpy(s->gen,f+4,8);s->offer=s->cap;s->selected=s->cap<u16(f+16)?s->cap:u16(f+16);s->peer_capacity=u16(f+18);s->pending_open=1;s->phase=BS_OPENING;return 0;}
 if(type==1){uint16_t offer=u16(f+16),capacity=u16(f+18);if(!s->server||!validcap(offer)||!validwindow(capacity))return fail(s,-2);if(s->phase==BS_OPEN||s->phase==BS_OPENING){if(s->offer!=offer||s->peer_capacity!=capacity)return fail(s,-2);s->pending_openack=1;return 0;}if(s->phase!=BS_INIT)return fail(s,-2);s->offer=offer;s->selected=offer<s->cap?offer:s->cap;s->peer_capacity=capacity;s->peer_limit=capacity;s->limit=s->capacity;s->phase=BS_OPENING;s->pending_openack=1;return 0;}
 if(type==2){if(s->server||(s->phase!=BS_OPENING&&s->phase!=BS_OPEN)||u16(f+16)!=s->selected||u16(f+18)!=s->peer_capacity)return fail(s,-2);if(s->phase==BS_OPENING){s->phase=BS_OPEN;s->peer_limit=s->peer_capacity;s->limit=s->capacity;s->pending_open=0;s->due=0;}return 0;}
 if(type==5){if(u16(f+16)>7)return fail(s,-2);s->reason=u16(f+16);s->phase=BS_CLOSING;s->pending_close=0;s->pending_closeack=1;s->close_at=now;return 0;}
 if(type==6){if(s->phase!=BS_CLOSING||u16(f+16)!=s->reason)return fail(s,-2);return fail(s,0);}
 if(s->phase!=BS_OPEN)return fail(s,-2);
 if(type==3){size_t len=n-16;uint64_t end=(uint64_t)off+len;if(!len||n>s->selected||end>BS_MAX)return fail(s,-3);if(end<=s->next){ack(s,now,1);return 0;}if(off<s->next||end>s->limit)return fail(s,-3);if(off>s->next){ack(s,now,1);return 0;}for(size_t i=0;i<len;i++)s->rx[(off+(uint32_t)i)%BS_CAP]=f[16+i];s->next=(uint32_t)end;ack(s,now,0);return 0;}
 if(type==4){uint32_t lim=u32(f+16);if(off<s->acked)return 0;if(off>s->dispatched||lim<off||(uint64_t)lim>(uint64_t)off+s->peer_capacity||lim>BS_MAX)return fail(s,-4);if(off>s->acked){int boundary=0;for(unsigned i=0;i<s->nflight;i++)if(s->flight[i].off+s->flight[i].len==off)boundary=1;if(!boundary)return fail(s,-4);unsigned remove=0;while(remove<s->nflight&&s->flight[remove].off+s->flight[remove].len<=off)remove++;for(uint32_t p=s->acked;p<off;p++)s->tx[p%BS_CAP]=0;memmove(s->flight,s->flight+remove,(s->nflight-remove)*sizeof(bs_desc));s->nflight=(uint8_t)(s->nflight-remove);s->acked=off;s->progress=now;s->retries=0;s->due=s->nflight?now+750:0;}if(lim>s->peer_limit){s->peer_limit=lim;s->blocked=0;}return 0;}
 return fail(s,-2);
}
size_t bs_write(bs_stream*s,const uint8_t*p,size_t n,uint64_t now){if(s->phase!=BS_OPEN||!p||!n)return 0;if(!clock_ok(s,now))return 0;if(s->end>BS_MAX||n>BS_MAX-s->end){fail(s,-5);return 0;}size_t free=BS_CAP-(s->end-s->acked);if(n>free)n=free;for(size_t i=0;i<n;i++)s->tx[(s->end+(uint32_t)i)%BS_CAP]=p[i];s->end+=(uint32_t)n;return n;}
size_t bs_read(bs_stream*s,uint8_t*p,size_t n,uint64_t now){if(s->phase!=BS_OPEN||!p)return 0;if(!clock_ok(s,now))return 0;size_t available=s->next-s->read;if(n>available)n=available;for(size_t i=0;i<n;i++){p[i]=s->rx[(s->read+(uint32_t)i)%BS_CAP];s->rx[(s->read+(uint32_t)i)%BS_CAP]=0;}s->read+=(uint32_t)n;uint64_t lim=(uint64_t)s->read+s->capacity;s->limit=lim>BS_MAX?BS_MAX:(uint32_t)lim;if(n)ack(s,now,0);return n;}
size_t bs_next(bs_stream*s,uint8_t out[BS_FRAME],uint64_t now){
 if(s->phase==BS_CLOSED)return 0;
 if(!clock_ok(s,now)||s->local)return 0;
 size_t n=0;uint8_t type=0;
 if(s->pending_close){type=5;s->pending_close=0;head(s,out,type,0);p16(out+16,s->reason);n=18;}
 else if(s->pending_closeack){type=6;s->pending_closeack=0;head(s,out,type,0);p16(out+16,s->reason);n=18;}
 else if(s->pending_openack){type=2;s->pending_openack=0;head(s,out,type,0);p16(out+16,s->selected);p16(out+18,s->capacity);n=20;s->phase=BS_OPEN;}
 else if(s->pending_open){type=1;s->pending_open=0;head(s,out,type,0);p16(out+16,s->offer);p16(out+18,s->capacity);n=20;s->due=now+(s->open_retries==0?750:s->open_retries==1?1500:3000);}
 else if(s->phase==BS_OPEN&&s->pending_ack&&now>=s->ack_due){type=4;s->pending_ack=0;head(s,out,type,s->next);p32(out+16,s->limit);n=20;}
 else if(s->phase==BS_OPEN){bs_desc d={0,0};int found=0;if(s->replay){for(unsigned i=0;i<s->nflight;i++)if(s->flight[i].off>=s->replay_at){d=s->flight[i];found=1;s->replay_at=d.off+d.len;break;}if(!found)s->replay=0;}
 if(!found&&s->dispatched<s->end&&s->nflight<BS_FLIGHT&&s->dispatched<s->peer_limit){uint32_t len=s->end-s->dispatched;if(len>s->selected-16u)len=s->selected-16u;if(len>s->peer_limit-s->dispatched)len=s->peer_limit-s->dispatched;d.off=s->dispatched;d.len=(uint16_t)len;s->flight[s->nflight++]=d;s->dispatched+=len;found=1;if(s->nflight==1){s->progress=now;s->due=now+750;s->retries=0;}}
 if(found){type=3;head(s,out,type,d.off);for(unsigned i=0;i<d.len;i++)out[16+i]=s->tx[(d.off+i)%BS_CAP];n=16+d.len;}}
 if(n){if(s->local_token==UINT32_MAX){fail(s,-5);return 0;}s->local_token++;s->local=1;s->local_type=type;s->local_at=now;}return n;
}
void bs_settle(bs_stream*s,const uint8_t gen[8],uint32_t token,int error,uint64_t now){if(s->phase==BS_CLOSED||memcmp(gen,s->gen,8)||token!=s->local_token||!s->local)return;if(!clock_ok(s,now))return;s->local=0;if(error){fail(s,-6);return;}if(s->local_type==6)fail(s,0);}
void bs_tick(bs_stream*s,uint64_t now){if(s->phase==BS_CLOSED)return;if(!clock_ok(s,now))return;if(s->local&&now-s->local_at>=5000){fail(s,-7);return;}if(s->phase==BS_CLOSING){if(now-s->close_at>=300)fail(s,0);return;}if((s->phase==BS_INIT||s->phase==BS_OPENING)&&now-s->start>=30000){fail(s,-7);return;}if(!s->server&&s->phase==BS_OPENING&&!s->pending_open&&s->due&&now>=s->due){if(s->open_retries++>=6){fail(s,-7);return;}s->pending_open=1;s->due=now+3000;}
 if(s->phase==BS_OPEN&&s->nflight){if(now-s->progress>=15000){fail(s,-7);return;}if(s->due&&now>=s->due){if(s->retries++>=6){fail(s,-7);return;}s->replay=1;s->replay_at=s->acked;uint64_t rto=s->retries==1?1500:3000;s->due=now+rto;}}
 if(s->phase==BS_OPEN&&s->end>s->dispatched&&s->peer_limit<=s->dispatched){if(!s->blocked)s->blocked=now?now:1;else if(now>=s->blocked&&now-s->blocked>=15000)fail(s,-7);}else s->blocked=0;
}
void bs_close(bs_stream*s,uint16_t reason,uint64_t now){if(s->phase==BS_CLOSED)return;if(!clock_ok(s,now))return;if(reason>7){fail(s,-2);return;}s->phase=BS_CLOSING;s->reason=reason;s->pending_close=1;s->close_at=now;}
