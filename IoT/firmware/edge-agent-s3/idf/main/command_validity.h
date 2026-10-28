#pragma once
#include <cstdint>
#include <string>
#include <sys/time.h>

namespace beefiscal::idf {
struct CommandInstant { int64_t seconds{}; uint32_t nanoseconds{}; };
// Explicit clock injection is reserved for host tests; production uses UTC realtime.
struct CommandClock { bool (*read)(CommandInstant&,void*){}; void* context{}; };
inline bool command_instant_less(const CommandInstant&a,const CommandInstant&b) {
  return a.seconds<b.seconds||(a.seconds==b.seconds&&a.nanoseconds<b.nanoseconds);
}
// The signed Go command producer emits UTC time.RFC3339Nano. Do not normalize
// impossible calendar dates, leap seconds, offsets or excess fractional digits.
inline bool parse_command_instant(const std::string&s,CommandInstant&out) {
  if(s.size()<20||s.size()>30||s[4]!='-'||s[7]!='-'||s[10]!='T'||s[13]!=':'||s[16]!=':'||s.back()!='Z')return false;
  auto digits=[&](size_t at,size_t count,int&v){v=0;for(size_t i=at;i<at+count;++i){if(s[i]<'0'||s[i]>'9')return false;v=v*10+s[i]-'0';}return true;};
  int year,month,day,hour,minute,second;
  if(!digits(0,4,year)||!digits(5,2,month)||!digits(8,2,day)||!digits(11,2,hour)||!digits(14,2,minute)||!digits(17,2,second)||year<1||month<1||month>12||hour>23||minute>59||second>59)return false;
  const int lengths[]={31,28,31,30,31,30,31,31,30,31,30,31};
  const bool leap=year%4==0&&(year%100!=0||year%400==0);
  if(day<1||day>lengths[month-1]+(month==2&&leap?1:0))return false;
  uint32_t nanos=0;
  if(s.size()!=20){if(s[19]!='.'||s.size()<22)return false;const size_t count=s.size()-21;for(size_t i=0;i<count;++i){char c=s[20+i];if(c<'0'||c>'9')return false;nanos=nanos*10+uint32_t(c-'0');}for(size_t i=count;i<9;++i)nanos*=10;}
  // Gregorian civil date to Unix days, independent of host timezone/time_t width.
  int y=year-(month<=2);const int era=y/400;const unsigned yoe=unsigned(y-era*400);
  const unsigned doy=(153*unsigned(month+(month>2?-3:9))+2)/5+unsigned(day)-1;
  const unsigned doe=yoe*365+yoe/4-yoe/100+doy;
  const int64_t days=int64_t(era)*146097+doe-719468;
  out={days*86400+hour*3600+minute*60+second,nanos};return true;
}
inline bool command_now(const CommandClock&clock,CommandInstant&out) {
  if(clock.read){if(!clock.read(out,clock.context))return false;}
  else {timeval tv{};if(gettimeofday(&tv,nullptr)!=0||tv.tv_usec<0||tv.tv_usec>=1000000)return false;out={int64_t(tv.tv_sec),uint32_t(tv.tv_usec)*1000};}
  return out.seconds>0&&out.nanoseconds<1000000000;
}
inline bool command_valid_now(const CommandInstant&issued,const CommandInstant&expires,const CommandClock&clock) {
  CommandInstant now;
  return command_instant_less(issued,expires)&&command_now(clock,now)&&!command_instant_less(now,issued)&&command_instant_less(now,expires);
}
}
