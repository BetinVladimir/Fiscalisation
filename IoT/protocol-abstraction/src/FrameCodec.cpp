#include "FrameCodec.hpp"
#include <algorithm>
namespace bee { namespace {
uint16_t sum(const std::vector<uint8_t>&v,size_t a,size_t b){uint16_t x=0;for(size_t i=a;i<=b;i++)x=uint16_t(x+v[i]);return x;}
void nibbleWord(std::vector<uint8_t>&v,uint16_t x){for(int n:{12,8,4,0})v.push_back(uint8_t(((x>>n)&15)+0x30));}
bool unNibbleWord(const std::vector<uint8_t>&v,size_t at,uint16_t&out){if(at+4>v.size())return false;out=0;for(size_t i=0;i<4;i++){if(v[at+i]<0x30||v[at+i]>0x3f)return false;out=uint16_t((out<<4)|(v[at+i]-0x30));}return true;}
}}
namespace bee {
std::vector<uint8_t>DaisyCodec::encode(uint8_t seq,uint8_t cmd,const std::vector<uint8_t>&data){if(data.size()>200)return{};std::vector<uint8_t>v{1,uint8_t(data.size()+0x24),seq,cmd};v.insert(v.end(),data.begin(),data.end());v.push_back(5);nibbleWord(v,sum(v,1,v.size()-1));v.push_back(3);return v;}
bool DaisyCodec::decode(const std::vector<uint8_t>&v,ParsedFrame&o,std::string&e){if(v.size()<17||v[0]!=1||v.back()!=3){e="DAISY_BAD_FRAME";return false;}if(v[1]<0x2b||v.size()>217){e="DAISY_LENGTH";return false;}size_t dataLen=size_t(v[1]-0x2b);if(v.size()!=17+dataLen){e="DAISY_LENGTH";return false;}size_t sep=4+dataLen,pst=11+dataLen;if(v[sep]!=4||v[pst]!=5){e="DAISY_SEPARATOR";return false;}uint16_t got;if(!unNibbleWord(v,pst+1,got)||got!=sum(v,1,pst)){e="DAISY_BCC";return false;}o.sequence=v[2];o.command=v[3];o.data.assign(v.begin()+4,v.begin()+sep);o.status.assign(v.begin()+sep+1,v.begin()+pst);return true;}
std::vector<uint8_t>DatecsCodec::encode(uint8_t seq,uint16_t cmd,const std::vector<uint8_t>&data){if(data.size()>496)return{};std::vector<uint8_t>v{1};nibbleWord(v,uint16_t(data.size()+0x2a));v.push_back(seq);nibbleWord(v,cmd);v.insert(v.end(),data.begin(),data.end());v.push_back(5);nibbleWord(v,sum(v,1,v.size()-1));v.push_back(3);return v;}
bool DatecsCodec::decode(const std::vector<uint8_t>&v,ParsedFrame&o,std::string&e){if(v.size()<25||v[0]!=1||v.back()!=3){e="DATECS_BAD_FRAME";return false;}uint16_t encoded;if(!unNibbleWord(v,1,encoded)||encoded<0x33||encoded>0x213){e="DATECS_LENGTH";return false;}size_t raw=encoded-0x20,dataLen=raw-19;if(v.size()!=25+dataLen){e="DATECS_LENGTH";return false;}size_t sep=10+dataLen,pst=19+dataLen;if(v[sep]!=4||v[pst]!=5){e="DATECS_SEPARATOR";return false;}uint16_t got,cmd;if(!unNibbleWord(v,pst+1,got)||got!=sum(v,1,pst)||!unNibbleWord(v,6,cmd)){e="DATECS_BCC";return false;}o.sequence=v[5];o.command=cmd;o.data.assign(v.begin()+10,v.begin()+sep);o.status.assign(v.begin()+sep+1,v.begin()+pst);return true;}
}
