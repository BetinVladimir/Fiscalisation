#include "FrameCodec.hpp"
#include <iostream>
#include <string>
#include <vector>
using Bytes = std::vector<uint8_t>;
static int failures = 0;
static void check(bool ok, const char* name) { if (!ok) { std::cerr << "FAIL " << name << '\n'; ++failures; } }
int main() {
    // Daisy v2.0-4 (2026), p7, status command example; literal vendor bytes.
    const Bytes daisyTx{0x01,0x24,0x50,0x4a,0x05,0x30,0x30,0x3c,0x33,0x03};
    const Bytes daisyRx{0x01,0x31,0x50,0x4a,0x88,0x80,0x80,0x80,0x80,0xb8,0x04,0x88,0x80,0x80,0x80,0x80,0xb8,0x05,0x30,0x37,0x35,0x34,0x03};
    // Datecs PM v2.11.4 section4.1 command33 example, literal PDF TX/RX.
    const Bytes datecsTx{0x01,0x30,0x30,0x32,0x3a,0x24,0x30,0x30,0x32,0x31,0x05,0x30,0x31,0x3b,0x38,0x03};
    const Bytes datecsRx{0x01,0x30,0x30,0x33,0x35,0x24,0x30,0x30,0x32,0x31,0x30,0x09,0x04,0x80,0x80,0x80,0x80,0x86,0x9a,0x80,0x80,0x05,0x30,0x36,0x31,0x31,0x03};
    check(bee::DaisyCodec::encode(0x50,0x4a,{}) == daisyTx,"Daisy vendor TX LEN/BCC");
    check(bee::DatecsCodec::encode(0x24,0x21,{}) == datecsTx,"Datecs vendor TX LEN/BCC");
    bee::ParsedFrame out; std::string error;
    check(bee::DaisyCodec::decode(daisyRx,out,error) && out.command==0x4a && out.sequence==0x50 && out.data==Bytes({0x88,0x80,0x80,0x80,0x80,0xb8}) && out.status==out.data,"Daisy vendor RX");
    check(bee::DatecsCodec::decode(datecsRx,out,error) && out.command==0x21 && out.sequence==0x24 && out.data==Bytes({'0','\t'}) && out.status==Bytes({0x80,0x80,0x80,0x80,0x86,0x9a,0x80,0x80}),"Datecs vendor RX");
    for (int vendor=0; vendor<2; ++vendor) {
        const Bytes& good=vendor?datecsRx:daisyRx;
        auto decode=[&](const Bytes& v){return vendor?bee::DatecsCodec::decode(v,out,error):bee::DaisyCodec::decode(v,out,error);};
        for(size_t n=0;n<good.size();++n) check(!decode(Bytes(good.begin(),good.begin()+n)),"truncated response rejected");
        for(size_t i=0;i<good.size();++i){auto bad=good;bad[i]^=1;check(!decode(bad),"single-byte corruption rejected");}
        auto bad=good;bad.push_back(0x03);check(!decode(bad),"trailing bytes rejected");
        bad=good;bad[1]=0x20;check(!decode(bad),"underflow LEN rejected");
        bad=good;bad[good.size()-3]='A';check(!decode(bad),"ASCII hex checksum rejected");
    }
    // Bounds from Daisy table p6 DATA 0..200; Datecs sec3.3/3.4 496/480.
    check(bee::DaisyCodec::encode(0x20,0x30,Bytes(200,'x')).size()==210,"Daisy max TX");
    check(bee::DaisyCodec::encode(0x20,0x30,Bytes(201,'x')).empty(),"Daisy oversize TX");
    auto max=bee::DatecsCodec::encode(0xff,0x21,Bytes(496,'x'));
    check(max.size()==512 && Bytes(max.begin()+1,max.begin()+5)==Bytes({0x30,0x32,0x31,0x3a}),"Datecs max TX LEN");
    check(bee::DatecsCodec::encode(0xff,0x21,Bytes(497,'x')).empty(),"Datecs oversize TX");
    // Independent response boundary construction from documented layouts, not production encode.
    auto response=[](bool datecs,size_t n) {
        Bytes v{1};
        auto word=[&](unsigned x){for(int shift:{12,8,4,0})v.push_back(uint8_t(0x30+((x>>shift)&15)));};
        if(datecs){word(unsigned(n+0x33));v.push_back(0x20);word(0x21);}
        else {v.push_back(uint8_t(n+0x2b));v.push_back(0x20);v.push_back(0x4a);}
        v.insert(v.end(),n,'x');v.push_back(4);v.insert(v.end(),datecs?8:6,0x80);v.push_back(5);
        unsigned checksum=0;for(size_t i=1;i<v.size();++i)checksum+=v[i];word(checksum&0xffff);v.push_back(3);return v;
    };
    check(bee::DaisyCodec::decode(response(false,200),out,error)&&out.data.size()==200,"Daisy max RX");
    check(!bee::DaisyCodec::decode(response(false,201),out,error),"Daisy oversize RX");
    check(bee::DatecsCodec::decode(response(true,480),out,error)&&out.data.size()==480,"Datecs max RX");
    check(!bee::DatecsCodec::decode(response(true,481),out,error),"Datecs oversize RX");
    // Deterministic adversarial byte streams: parser must safely reject noise.
    for(size_t n=0;n<550;++n) { Bytes noise(n,0xff);check(!bee::DaisyCodec::decode(noise,out,error),"Daisy noise");check(!bee::DatecsCodec::decode(noise,out,error),"Datecs noise"); }
    // Golden answer frame: cmd 48 (sale), seq 55, UNP DY000694-OP01-0000018; literal bytes from fixture.
    {bee::ParsedFrame out48;std::string err48;
     const Bytes golden48{0x01,0x38,0x37,0x30,0x30,0x30,0x30,0x30,0x30,0x31,0x2c,0x30,0x30,0x30,0x30,0x30,0x30,0x04,0x88,0x80,0x88,0x80,0x80,0xb8,0x05,0x30,0x36,0x35,0x3d,0x03};
     check(bee::DaisyCodec::decode(golden48,out48,err48)&&out48.command==0x30&&out48.sequence==0x37&&out48.data==Bytes{0x30,0x30,0x30,0x30,0x30,0x31,0x2c,0x30,0x30,0x30,0x30,0x30,0x30}&&out48.status==Bytes{0x88,0x80,0x88,0x80,0x80,0xb8},"Daisy golden RX cmd48 DY000694");
     auto bad48=golden48;bad48[25]^=1;check(!bee::DaisyCodec::decode(bad48,out48,err48),"Daisy golden RX cmd48 bad BCC");}
    if(!failures)std::cout<<"golden_daisy_cmd48=pass\n";
    if(failures){std::cerr<<failures<<" failures\n";return 1;} std::cout<<"runtime vendor codec tests: OK\n";
}
