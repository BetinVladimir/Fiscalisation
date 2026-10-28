#include <cassert>
#include <cstdio>
#include <string>
#include "durable_storage.h"
using namespace beefiscal::idf;
int main(int argc,char**argv){assert(argc==2);remove(argv[1]);DurableStorage s;assert(s.open_database_for_test(argv[1])==ESP_OK);assert(s.reserve_with_received_event("op",std::string(64,'a').c_str(),"MQTT","{}","e1","{}",1704067200).status==ReservationStatus::New);assert(s.acknowledge_through(1,"signed-but-not-inflight",std::string(64,'b').c_str(),1704067201)!=ESP_OK);assert(s.acknowledged_cursor()==0);}
