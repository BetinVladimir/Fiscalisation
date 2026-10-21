#ifndef BF_STREAM_H
#define BF_STREAM_H
#include <stddef.h>
#include <stdint.h>
#ifdef __cplusplus
extern "C" {
#endif
#define BS_CAP 2048u
#define BS_FRAME 182u
#define BS_FLIGHT 16u
#define BS_MAX 0xffff0000u
enum {BS_INIT,BS_OPENING,BS_OPEN,BS_CLOSING,BS_CLOSED};
typedef struct {uint32_t off;uint16_t len;} bs_desc;
typedef struct {
 uint8_t tx[BS_CAP],rx[BS_CAP],gen[8];bs_desc flight[BS_FLIGHT];
 uint32_t acked,dispatched,end,read,next,limit,peer_limit;
 uint64_t start,last_tick,local_at,due,progress,blocked,ack_due,close_at;
 uint32_t replay_at,local_token;uint16_t cap,selected,capacity,peer_capacity,offer,reason;
 uint8_t server,phase,nflight,local,pending_open,pending_openack,pending_ack,pending_close,pending_closeack,replay,retries,open_retries,local_type;
 int error;
} bs_stream;
void bs_init(bs_stream*,int server,const uint8_t gen[8],uint16_t cap,uint16_t capacity,uint64_t now);
size_t bs_info(bs_stream*,uint8_t out[20]);
int bs_feed(bs_stream*,const uint8_t*,size_t,uint64_t now);
size_t bs_write(bs_stream*,const uint8_t*,size_t,uint64_t now);
size_t bs_read(bs_stream*,uint8_t*,size_t,uint64_t now);
size_t bs_next(bs_stream*,uint8_t out[BS_FRAME],uint64_t now);
void bs_settle(bs_stream*,const uint8_t generation[8],uint32_t token,int error,uint64_t now);
void bs_tick(bs_stream*,uint64_t now);
void bs_close(bs_stream*,uint16_t reason,uint64_t now);
#ifdef __cplusplus
}
#endif
#endif
