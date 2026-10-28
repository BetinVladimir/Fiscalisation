#include "serial_exchange.h"
#include "FrameCodec.hpp"
#include <algorithm>
namespace beefiscal::idf { namespace {
// Datecs PM v2.11.4 section6; Daisy v2.0-4 section5. State flags and
// reserved bit7 are not errors. Unknown model-specific Daisy error codes fail closed.
bool status_ok(SerialProfile profile,const std::vector<uint8_t>&s){
 if(profile==SerialProfile::Datecs)return s.size()==8&&!(s[0]&0x73)&&!(s[1]&0x03)&&!(s[2]&0x05)&&!(s[4]&0x71);
 return s.size()==6&&!(s[0]&0x33)&&!(s[1]&0x6f)&&!(s[2]&0x05)&&!(s[3]&0x7f)&&!(s[4]&0x37)&&!(s[5]&0x01);
}
bool result_ok(SerialProfile profile,const std::vector<uint8_t>&data){
 // Applicable Datecs PM commands return ErrorCode as their first tab-delimited field.
 return profile!=SerialProfile::Datecs||(data.size()>=2&&data[0]=='0'&&data[1]=='\t');
}
}
bool SerialSession::exchange(SerialProfile profile,const SerialTransport&io,uint16_t command,const std::vector<uint8_t>&payload,bee::ParsedFrame*out){
 if(!io.write||!io.read||!io.now_ms||(profile==SerialProfile::Daisy&&command>255))return false;
 const uint8_t sequence=next_sequence_;
 const auto request=profile==SerialProfile::Datecs?bee::DatecsCodec::encode(sequence,command,payload):bee::DaisyCodec::encode(sequence,uint8_t(command),payload);
 if(request.empty())return false;
 // Consume SEQ for every attempted logical command, including failures. Retries
 // below reuse the same immutable packet (Datecs3.3 / Daisy2-3 duplicate cache).
 next_sequence_=(profile==SerialProfile::Datecs)?(sequence==0x7f?0x20:uint8_t(sequence+1)):(sequence==0xff?0x20:uint8_t(sequence+1));
 const int64_t deadline=io.now_ms(io.context)+(profile==SerialProfile::Daisy?3000:6000);
 int64_t receive_deadline=0;unsigned sends=0;
 auto send=[&](){
  if(sends==3||io.now_ms(io.context)>=deadline)return false;
  ++sends;
  const uint32_t timeout=uint32_t(std::min<int64_t>(1000,deadline-io.now_ms(io.context)));
  if(io.write(io.context,request.data(),request.size(),timeout)!=int(request.size()))return false;
  receive_deadline=io.now_ms(io.context)+500;return true;
 };
 if(!send())return false;
 std::vector<uint8_t> response;response.reserve(505);uint8_t chunk[128];
 while(io.now_ms(io.context)<deadline){
  const int64_t now=io.now_ms(io.context);
  if(now>=receive_deadline){response.clear();if(!send())return false;continue;}
  const uint32_t timeout=uint32_t(std::min(deadline,receive_deadline)-now);
  const int n=io.read(io.context,chunk,sizeof(chunk),timeout);
  if(n<0||n>int(sizeof(chunk))||io.now_ms(io.context)>=deadline)return false;
  if(n==0)continue;
  for(int i=0;i<n;++i){
   const uint8_t byte=chunk[i];
   if(response.empty()){
    if(byte==0x16){receive_deadline=io.now_ms(io.context)+500;continue;}
    if(byte==0x15){if(!send())return false;continue;}
    if(byte!=1)continue;
   }
   if(response.size()==505)return false;
   response.push_back(byte);
   if(byte!=3)continue;
   bee::ParsedFrame parsed;std::string error;
   const bool valid=profile==SerialProfile::Datecs?bee::DatecsCodec::decode(response,parsed,error):bee::DaisyCodec::decode(response,parsed,error);
   if(!valid)return false;
   if(parsed.sequence!=sequence||parsed.command!=command){response.clear();continue;}
   // A valid correlated frame consumed the command even if device reports error.
   if(!status_ok(profile,parsed.status)||!result_ok(profile,parsed.data))return false;
   if(out)*out=std::move(parsed);
   return true;
  }
 }
 return false;
}
}
