#include "cJSON.h"
#include <fstream>
#include <iostream>
#include <string>
int main(int argc,char**argv){if(argc!=2)return 2;std::ifstream f(argv[1]);std::string line;unsigned failed=0;while(std::getline(f,line)){if(line[0]!='M')continue;auto q=line.find('\t',2);auto* v=cJSON_Parse(line.substr(2,q-2).c_str());char*p=cJSON_PrintUnformatted(v);if(!p||std::string(p)!=line.substr(q+1))++failed;cJSON_free(p);cJSON_Delete(v);}std::cout<<"existing cJSON serialization mismatches="<<failed<<"\n";return failed?1:0;}
