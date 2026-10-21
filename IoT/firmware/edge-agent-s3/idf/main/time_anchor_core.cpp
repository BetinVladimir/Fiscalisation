#include "time_anchor_core.h"
#include <algorithm>
#include <limits>
#include <utility>
namespace beefiscal::idf {namespace {
constexpr int64_t million=1000000,minimum_epoch_ms=1700000000000;
bool add(int64_t a,int64_t b,int64_t& out){if(a<0||b<0||a>INT64_MAX-b)return false;out=a+b;return true;}
bool ms(int64_t value,int64_t& out){if(value<0||value>INT64_MAX/1000)return false;out=value*1000;return true;}
// Exact outward rational arithmetic without floating point or target int128.
bool scale(int64_t elapsed,int64_t denominator,bool upward,int64_t& out){
 if(elapsed<0||denominator<=0)return false;
 const auto q=elapsed/denominator,r=elapsed%denominator;
 if(q>INT64_MAX/million||r>INT64_MAX/million)return false;
 const auto numerator=r*million;auto tail=numerator/denominator;
 if(upward&&numerator%denominator)++tail;
 return add(q*million,tail,out);
}
bool hash(const std::string& s){return s.size()==64&&std::all_of(s.begin(),s.end(),[](char c){return(c>='0'&&c<='9')||(c>='a'&&c<='f');});}
bool text(const std::string& s){return !s.empty()&&s.size()<=256&&s.find('\0')==std::string::npos;}
bool zero(const TimeNonce& n){return std::all_of(n.begin(),n.end(),[](uint8_t b){return b==0;});}
}
bool TimeAnchorScope::operator==(const TimeAnchorScope& s)const{return device_id==s.device_id&&device_kid==s.device_kid&&device_spki_sha256==s.device_spki_sha256&&tenant_id==s.tenant_id&&location_id==s.location_id&&register_id==s.register_id&&binding_sha256==s.binding_sha256&&binding_generation==s.binding_generation;}
#if defined(BEEFISCAL_TIME_ANCHOR_HOST_TEST)
void TimeAnchorHostPermitFactory::sequence(TimeAnchorCore& core,int64_t value){std::lock_guard<std::mutex> lock(core.mutex_);core.sequence_=value;}
#endif
TimeAnchorCore::TimeAnchorCore(TimeAnchorMonotonicClock& c,TimeAnchorNonceSource& n,TimeAnchorPolicy p):clock_(c),nonces_(n),policy_(p){}
bool TimeAnchorCore::valid_policy()const{return policy_.max_rtt_ms>0&&policy_.max_rtt_ms<=60000&&policy_.max_holdover_ms>0&&policy_.max_holdover_ms<=3600000&&policy_.max_uncertainty_ms>=0&&policy_.max_uncertainty_ms<=60000&&policy_.max_width_ms>0&&policy_.max_width_ms<=3600000&&policy_.drift_ppm>=0&&policy_.drift_ppm<=100000&&policy_.nonce_capacity>0&&policy_.nonce_capacity<=history_.size();}
bool TimeAnchorCore::valid_scope(const TimeAnchorScope& s)const{return text(s.device_id)&&text(s.device_kid)&&hash(s.device_spki_sha256)&&text(s.tenant_id)&&text(s.location_id)&&text(s.register_id)&&hash(s.binding_sha256)&&s.binding_generation>0;}
void TimeAnchorCore::retire(bool quarantine){pending_={};has_anchor_=false;quarantined_=quarantine;}
TimeAnchorError TimeAnchorCore::read_clock(int64_t& now){if(!clock_.read_us(now)||now<0||(has_observed_&&now<observed_)){retire(true);return TimeAnchorError::ClockFault;}has_observed_=true;observed_=now;return TimeAnchorError::Ok;}
TimeAnchorError TimeAnchorCore::nonce(TimeNonce& out){
 out={};
 if(!nonces_.fill(out)||zero(out)||(boot_ready_&&out==boot_nonce_)||std::find(history_.begin(),history_.begin()+history_size_,out)!=history_.begin()+history_size_){out={};return TimeAnchorError::NonceFault;}
 history_[history_next_]=out;history_next_=(history_next_+1)%policy_.nonce_capacity;history_size_=std::min(history_size_+1,policy_.nonce_capacity);return TimeAnchorError::Ok;
}
TimeAnchorError TimeAnchorCore::start_boot(const TimeAnchorScope& scope){
 std::lock_guard<std::mutex> lock(mutex_);retire(false);boot_ready_=false;has_observed_=false;has_published_=false;sequence_=0;boot_nonce_={};
 if(!valid_policy())return TimeAnchorError::InvalidPolicy;
 if(!valid_scope(scope))return TimeAnchorError::InvalidScope;
 int64_t now{};auto e=read_clock(now);if(e!=TimeAnchorError::Ok)return e;
 e=nonce(boot_nonce_);if(e!=TimeAnchorError::Ok)return e;scope_=scope;boot_ready_=true;return TimeAnchorError::Ok;
}
TimeAnchorError TimeAnchorCore::change_binding(const TimeAnchorScope& scope){std::lock_guard<std::mutex> lock(mutex_);retire(false);has_published_=false;if(!boot_ready_)return TimeAnchorError::Unavailable;if(!valid_scope(scope)){boot_ready_=false;return TimeAnchorError::InvalidScope;}scope_=scope;return TimeAnchorError::Ok;}
void TimeAnchorCore::invalidate(){std::lock_guard<std::mutex> lock(mutex_);retire(false);boot_ready_=false;has_published_=false;boot_nonce_={};}
TimeAnchorError TimeAnchorCore::begin_request(TimeAnchorRequest& out){
 std::lock_guard<std::mutex> lock(mutex_);out={};pending_={};
 if(!boot_ready_||quarantined_)return TimeAnchorError::Unavailable;
 int64_t now{};auto e=read_clock(now);if(e!=TimeAnchorError::Ok)return e;
 if(sequence_==INT64_MAX||next_ticket_==INT64_MAX)return TimeAnchorError::SequenceExhausted;
 TimeNonce fresh{};e=nonce(fresh);if(e!=TimeAnchorError::Ok)return e;
 TimeAnchorRequest request;request.scope=scope_;request.boot_nonce=boot_nonce_;request.request_nonce=fresh;request.sequence=++sequence_;request.owner_=this;request.ticket_=++next_ticket_;pending_=request;sent_=now;out=request;return TimeAnchorError::Ok;
}
TimeAnchorError TimeAnchorCore::project(const Anchor& a,int64_t now,TimeAnchorBounds& out)const{
 out={};if(now<a.received||now<a.sent)return TimeAnchorError::ClockFault;
 int64_t age{},lo{},hi{};
 if(!scale(now-a.sent,million-policy_.drift_ppm,true,age))return TimeAnchorError::Arithmetic;
 if(age>=a.holdover)return TimeAnchorError::Expired;
 if(!scale(now-a.received,million+policy_.drift_ppm,false,lo)||!scale(now-a.received,million-policy_.drift_ppm,true,hi)||!add(a.lower,lo,out.lower_us)||!add(a.upper,hi,out.upper_us))return TimeAnchorError::Arithmetic;
 if(out.lower_us>out.upper_us)return TimeAnchorError::Arithmetic;
 if(out.upper_us-out.lower_us>policy_.max_width_ms*1000)return TimeAnchorError::WidthExceeded;
 return TimeAnchorError::Ok;
}
TimeAnchorError TimeAnchorCore::clamp(TimeAnchorBounds& out)const{
 if(has_published_){if(published_lower_>out.upper_us)return TimeAnchorError::Disjoint;out.lower_us=std::max(out.lower_us,published_lower_);out.upper_us=std::max(out.upper_us,published_upper_);}
 if(out.lower_us>out.upper_us)return TimeAnchorError::Arithmetic;
 if(out.upper_us-out.lower_us>policy_.max_width_ms*1000)return TimeAnchorError::WidthExceeded;
 return TimeAnchorError::Ok;
}
TimeAnchorError TimeAnchorCore::accept(const VerifiedTimeAnchorPermit& permit){
 std::lock_guard<std::mutex> lock(mutex_);
 if(!boot_ready_||quarantined_||!pending_.ticket_||permit.owner_!=this||permit.ticket_!=pending_.ticket_)return TimeAnchorError::StaleTicket;
 const auto& c=permit.claims_;
 // Foreign/stale permits cannot mutate the current pending request or anchor.
 if(!(c.scope==scope_)||!(c.scope==pending_.scope)||c.boot_nonce!=boot_nonce_||c.request_nonce!=pending_.request_nonce||c.sequence!=pending_.sequence)return TimeAnchorError::StaleTicket;
 int64_t now{};auto e=read_clock(now);if(e!=TimeAnchorError::Ok)return e;
 pending_={}; // Matching response consumes its ticket even on failure.
 if(!c.clock_healthy||c.server_unix_ms<minimum_epoch_ms||c.uncertainty_ms<0||c.uncertainty_ms>policy_.max_uncertainty_ms||c.max_rtt_ms<=0||c.max_rtt_ms>policy_.max_rtt_ms||c.max_holdover_ms<=0||c.max_holdover_ms>policy_.max_holdover_ms){retire(true);return TimeAnchorError::InvalidSample;}
 int64_t t{},error{},rtt_limit{},holdover{},rtt{};
 if(!ms(c.server_unix_ms,t)||!ms(c.uncertainty_ms,error)||!ms(c.max_rtt_ms,rtt_limit)||!ms(c.max_holdover_ms,holdover)||!scale(now-sent_,million-policy_.drift_ppm,true,rtt)||t<error){retire(true);return TimeAnchorError::Arithmetic;}
 if(rtt>rtt_limit)return TimeAnchorError::RoundTripExceeded;
 Anchor next;next.lower=t-error;next.sent=sent_;next.received=now;next.holdover=holdover;
 if(!add(t,error,next.upper)||!add(next.upper,rtt,next.upper)){retire(true);return TimeAnchorError::Arithmetic;}
 TimeAnchorBounds candidate{};e=project(next,now,candidate);if(e!=TimeAnchorError::Ok)return e;
 if(has_anchor_){TimeAnchorBounds old{};const auto previous=project(anchor_,now,old);if(previous==TimeAnchorError::Ok){if(candidate.upper_us<old.lower_us||candidate.lower_us>old.upper_us){retire(true);return TimeAnchorError::Disjoint;}}else if(previous!=TimeAnchorError::Expired){retire(true);return previous;}}
 e=clamp(candidate);if(e!=TimeAnchorError::Ok){retire(true);return e;}
 // Preserve a conservative nondecreasing publication floor across reanchors.
 next.lower=candidate.lower_us;next.upper=candidate.upper_us;anchor_=next;has_anchor_=true;return TimeAnchorError::Ok;
}
TimeAnchorError TimeAnchorCore::bounds(TimeAnchorBounds& out){
 std::lock_guard<std::mutex> lock(mutex_);out={};if(!boot_ready_||quarantined_||!has_anchor_)return TimeAnchorError::Unavailable;
 int64_t now{};auto e=read_clock(now);if(e!=TimeAnchorError::Ok)return e;
 TimeAnchorBounds current{};e=project(anchor_,now,current);if(e==TimeAnchorError::Ok)e=clamp(current);
 if(e!=TimeAnchorError::Ok){if(e==TimeAnchorError::Expired)has_anchor_=false;else retire(true);return e;}
 published_lower_=current.lower_us;published_upper_=current.upper_us;has_published_=true;out=current;return TimeAnchorError::Ok;
}
}
