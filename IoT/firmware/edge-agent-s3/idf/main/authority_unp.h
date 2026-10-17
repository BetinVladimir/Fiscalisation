#pragma once
#include <cstdint>
#include <string>
namespace beefiscal::idf {
inline bool authority_prefix_valid(const std::string&p){if(p.size()!=8)return false;for(size_t i=0;i<8;++i){const unsigned char c=p[i];if(i<2?(c<'A'||c>'Z'):(c<'0'||c>'9'))return false;}return true;}
inline bool authority_operator_valid(const std::string&op){if(op.size()!=4)return false;for(unsigned char c:op)if(!((c>='A'&&c<='Z')||(c>='a'&&c<='z')||(c>='0'&&c<='9')))return false;return true;}
inline bool authority_unp(const std::string&serial,const std::string&op,int64_t sequence,std::string&out){out.clear();if(!authority_prefix_valid(serial)||!authority_operator_valid(op)||sequence<1||sequence>9999999)return false;const auto digits=std::to_string(sequence);out=serial+"-"+op+"-"+std::string(7-digits.size(),'0')+digits;return true;}
inline bool authority_unp_valid(const std::string&unp){if(unp.size()!=21||unp[8]!='-'||unp[13]!='-'||!authority_prefix_valid(unp.substr(0,8))||!authority_operator_valid(unp.substr(9,4)))return false;bool nonzero=false;for(size_t i=14;i<21;++i){if(unp[i]<'0'||unp[i]>'9')return false;nonzero|=unp[i]!='0';}return nonzero;}
inline int64_t authority_unp_sequence(const std::string&unp){if(!authority_unp_valid(unp))return 0;int64_t n=0;for(size_t i=14;i<21;++i)n=n*10+unp[i]-'0';return n;}
}
