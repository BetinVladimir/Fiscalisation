#pragma once
#include <array>
#include <cstddef>
#include <cstdint>
#include <mutex>
#include <string>
namespace beefiscal::idf {
using TimeNonce=std::array<uint8_t,32>;
struct TimeAnchorScope {
 std::string device_id,device_kid,device_spki_sha256,tenant_id,location_id,register_id,binding_sha256;
 int64_t binding_generation{};
 bool operator==(const TimeAnchorScope&)const;
};
class TimeAnchorNonceSource {public:virtual ~TimeAnchorNonceSource()=default;virtual bool fill(TimeNonce&)=0;};
class TimeAnchorMonotonicClock {public:virtual ~TimeAnchorMonotonicClock()=default;virtual bool read_us(int64_t&)const=0;};
// Fixed recent nonce ring detects stuck/repeated RNG output; retired entries may
// be evicted. Active boot nonce is always excluded; sequence/ticket prevent replay.
// max_rtt_ms bounds conservative real elapsed, including drift and verification.
// Engineering ceilings only. A production drift/health contract needs separate HIL.
struct TimeAnchorPolicy {int64_t max_rtt_ms=5000,max_holdover_ms=30000,max_uncertainty_ms=250,max_width_ms=6000,drift_ppm=1000;size_t nonce_capacity=64;};
struct TimeAnchorBounds {int64_t lower_us{},upper_us{};};
enum class TimeAnchorError {Ok,Unavailable,InvalidPolicy,InvalidScope,ClockFault,NonceFault,SequenceExhausted,StaleTicket,InvalidSample,RoundTripExceeded,Expired,WidthExceeded,Arithmetic,Disjoint};
class TimeAnchorCore;
class DeviceTimeAnchorVerifier; // Future strict cryptographic verifier; not implemented here.
struct TimeAnchorRequest {
 TimeAnchorScope scope;TimeNonce boot_nonce{},request_nonce{};int64_t sequence{};
 private:friend class TimeAnchorCore;friend class DeviceTimeAnchorVerifier;friend class VerifiedTimeAnchorPermit;
#if defined(BEEFISCAL_TIME_ANCHOR_HOST_TEST)
 friend class TimeAnchorHostPermitFactory;
#endif
 const TimeAnchorCore* owner_{};int64_t ticket_{};
};
// Exact signed int64 fields after future strict canonical/crypto verification.
// No global binary64 ceiling: opaque JS relays must not parse canonical numbers.
struct TimeAnchorClaims {TimeAnchorScope scope;TimeNonce boot_nonce{},request_nonce{};int64_t sequence{},server_unix_ms{},uncertainty_ms{},max_rtt_ms{},max_holdover_ms{};bool clock_healthy=false;};
class VerifiedTimeAnchorPermit final {
 public:VerifiedTimeAnchorPermit(const VerifiedTimeAnchorPermit&)=default;
 private:friend class TimeAnchorCore;friend class DeviceTimeAnchorVerifier;
#if defined(BEEFISCAL_TIME_ANCHOR_HOST_TEST)
 friend class TimeAnchorHostPermitFactory;
#endif
 VerifiedTimeAnchorPermit(const TimeAnchorRequest& request,const TimeAnchorClaims& claims):owner_(request.owner_),ticket_(request.ticket_),claims_(claims){}
 const TimeAnchorCore* owner_{};int64_t ticket_{};TimeAnchorClaims claims_;
};
#if defined(BEEFISCAL_TIME_ANCHOR_HOST_TEST)
class TimeAnchorHostPermitFactory final {public:static VerifiedTimeAnchorPermit make(const TimeAnchorRequest& request,const TimeAnchorClaims& claims){return {request,claims};}static void sequence(TimeAnchorCore&,int64_t);};
#endif
class TimeAnchorCore final {
 public:
 TimeAnchorCore(TimeAnchorMonotonicClock&,TimeAnchorNonceSource&,TimeAnchorPolicy={});
 TimeAnchorError start_boot(const TimeAnchorScope&); // Trusted lifecycle only; fresh boot nonce, no NVS restore.
 TimeAnchorError change_binding(const TimeAnchorScope&); // Retires anchor+pending even for A→B→A.
 void invalidate();
 TimeAnchorError begin_request(TimeAnchorRequest&); // m0 before returned nonce; only old pending retired.
 TimeAnchorError accept(const VerifiedTimeAnchorPermit&); // m1 after future verifier; no raw time setter.
 TimeAnchorError bounds(TimeAnchorBounds&); // Inclusive µs; outputs cleared on failure.
 private:
#if defined(BEEFISCAL_TIME_ANCHOR_HOST_TEST)
 friend class TimeAnchorHostPermitFactory;
#endif
 struct Anchor {int64_t lower{},upper{},sent{},received{},holdover{};};
 TimeAnchorMonotonicClock& clock_;TimeAnchorNonceSource& nonces_;TimeAnchorPolicy policy_;std::mutex mutex_;
 bool boot_ready_=false,quarantined_=false,has_anchor_=false,has_observed_=false,has_published_=false;
 int64_t observed_{},sequence_{},next_ticket_{},sent_{},published_lower_{},published_upper_{};TimeAnchorScope scope_;TimeNonce boot_nonce_{};
 std::array<TimeNonce,128> history_{};size_t history_size_{},history_next_{};TimeAnchorRequest pending_;Anchor anchor_;
 bool valid_policy()const;bool valid_scope(const TimeAnchorScope&)const;
 TimeAnchorError read_clock(int64_t&);TimeAnchorError nonce(TimeNonce&);
 void retire(bool quarantine);TimeAnchorError project(const Anchor&,int64_t,TimeAnchorBounds&)const;
 TimeAnchorError clamp(TimeAnchorBounds&)const;
};
}
