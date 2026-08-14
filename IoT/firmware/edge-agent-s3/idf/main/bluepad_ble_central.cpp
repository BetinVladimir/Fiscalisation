#include "bluepad_ble_central.h"
#include "host/ble_gap.h"
#include "host/ble_gatt.h"
#include "host/ble_hs.h"
#include "host/ble_hs_adv.h"
#include "host/ble_uuid.h"
#include "freertos/FreeRTOS.h"
#include "freertos/semphr.h"
#include <atomic>
#include <cstring>
#include <vector>

namespace beefiscal::idf{namespace{
constexpr uint8_t START=0x3e,CMD_BORICA=0x3d,EVT_BORICA=0x0e;
constexpr uint8_t TX_START=0x01,TX_END=0x03,GET_REPORT_BY_STAN=0x24;
constexpr uint8_t PURCHASE=0x01,VOID_PURCHASE=0x07;
constexpr uint16_t TAG_AMOUNT=0x81,TAG_RRN=0xdf01,TAG_AUTH=0xdf02,
 TAG_RESULT=0xdf05,TAG_ERROR=0xdf06,TAG_HOST_RRN=0xdf07,
 TAG_HOST_AUTH=0xdf08,TAG_STAN=0x9f41;
PaymentEndpointBinding cfg;std::atomic_bool connected{false},configured{false};
uint8_t own_addr_type{};uint16_t connection=BLE_HS_CONN_HANDLE_NONE;
uint16_t service_start{},service_end{},tx_handle{},rx_handle{},cccd_handle{};
ble_uuid_any_t service_uuid{},tx_uuid{},rx_uuid{};
SemaphoreHandle_t response_sem{};std::vector<uint8_t>rx_bytes;PaymentOutcome outcome;

uint8_t checksum(const uint8_t*p,size_t n){uint8_t v=0;for(size_t i=0;i<n;i++)v^=p[i];return v;}
void append_tlv(std::vector<uint8_t>&v,uint16_t tag,const uint8_t*p,size_t n){if(tag>0xff)v.push_back(tag>>8);v.push_back(tag);v.push_back(n);v.insert(v.end(),p,p+n);}
std::vector<uint8_t> packet(uint8_t sub,uint8_t type,int64_t amount,const std::string&rrn={}){
 std::vector<uint8_t>d{sub,type};uint8_t money[4]={(uint8_t)(amount>>24),(uint8_t)(amount>>16),(uint8_t)(amount>>8),(uint8_t)amount};append_tlv(d,TAG_AMOUNT,money,4);
 if(!rrn.empty())append_tlv(d,TAG_RRN,(const uint8_t*)rrn.data(),rrn.size());
 std::vector<uint8_t>p{START,CMD_BORICA,0,(uint8_t)(d.size()>>8),(uint8_t)d.size()};p.insert(p.end(),d.begin(),d.end());p.push_back(checksum(p.data(),p.size()));return p;
}
std::vector<uint8_t> lookup_packet(const std::string&reference){uint32_t stan=0;for(char c:reference)if(c>='0'&&c<='9')stan=stan*10+(c-'0');std::vector<uint8_t>d{GET_REPORT_BY_STAN,(uint8_t)(stan>>24),(uint8_t)(stan>>16),(uint8_t)(stan>>8),(uint8_t)stan,(uint8_t)(TAG_RESULT>>8),(uint8_t)TAG_RESULT,(uint8_t)(TAG_ERROR>>8),(uint8_t)TAG_ERROR,(uint8_t)(TAG_HOST_RRN>>8),(uint8_t)TAG_HOST_RRN,(uint8_t)(TAG_HOST_AUTH>>8),(uint8_t)TAG_HOST_AUTH};std::vector<uint8_t>p{START,CMD_BORICA,0,(uint8_t)(d.size()>>8),(uint8_t)d.size()};p.insert(p.end(),d.begin(),d.end());p.push_back(checksum(p.data(),p.size()));return p;}
bool tlv(const uint8_t*p,size_t n,uint16_t wanted,const uint8_t*&value,size_t&length){for(size_t i=0;i<n;){uint16_t tag=p[i++];if((tag==0x9f||tag==0xdf)&&i<n)tag=(tag<<8)|p[i++];if(i>=n)return false;size_t z=p[i++];if(i+z>n)return false;if(tag==wanted){value=p+i;length=z;return true;}i+=z;}return false;}
uint32_t number(const uint8_t*p,size_t n){uint32_t v=0;for(size_t i=0;i<n&&i<4;i++)v=(v<<8)|p[i];return v;}
std::string string_value(const uint8_t*p,size_t n){return std::string((const char*)p,n);}
void consume(){while(rx_bytes.size()>=6){if(rx_bytes[0]!=START){rx_bytes.erase(rx_bytes.begin());continue;}size_t n=(size_t(rx_bytes[3])<<8)|rx_bytes[4],total=6+n;if(rx_bytes.size()<total)return;if(checksum(rx_bytes.data(),total-1)!=rx_bytes[total-1]){rx_bytes.erase(rx_bytes.begin());continue;}uint8_t command=rx_bytes[1],status=rx_bytes[2];const uint8_t*data=rx_bytes.data()+5;if((command==EVT_BORICA&&n>1&&(data[0]==1||data[0]==2))||(command==0&&status==0&&n>0)){const bool event=command==EVT_BORICA;const uint8_t*fields=data+(event?1:0);size_t fields_size=n-(event?1:0);const uint8_t*v=nullptr;size_t z=0;uint32_t result=1,error=0,stan=0;if(tlv(fields,fields_size,TAG_RESULT,v,z))result=number(v,z);if(tlv(fields,fields_size,TAG_ERROR,v,z))error=number(v,z);if(tlv(fields,fields_size,TAG_STAN,v,z))stan=number(v,z);outcome.certainty=result==0?PaymentCertainty::Approved:PaymentCertainty::Declined;if(stan)outcome.reference=std::to_string(stan);if(tlv(fields,fields_size,TAG_HOST_RRN,v,z))outcome.rrn=string_value(v,z);if(tlv(fields,fields_size,TAG_HOST_AUTH,v,z))outcome.auth_code=string_value(v,z);if(error)outcome.error_code="BLUEPAD_"+std::to_string(error);xSemaphoreGive(response_sem);}else if(command==0&&status!=0){outcome={PaymentCertainty::Declined,"","","","BLUEPAD_"+std::to_string(status)};xSemaphoreGive(response_sem);}rx_bytes.erase(rx_bytes.begin(),rx_bytes.begin()+total);}}
int descriptor_cb(uint16_t,const ble_gatt_error*e,uint16_t,const ble_gatt_dsc*d,void*){if(e->status==0&&d&&ble_uuid_u16(&d->uuid.u)==BLE_GATT_DSC_CLT_CFG_UUID16)cccd_handle=d->handle;if(e->status==BLE_HS_EDONE&&cccd_handle){uint8_t on[2]={1,0};ble_gattc_write_flat(connection,cccd_handle,on,2,nullptr,nullptr);}return 0;}
int characteristic_cb(uint16_t,const ble_gatt_error*e,const ble_gatt_chr*c,void*){if(e->status==0&&c){if(ble_uuid_cmp(&c->uuid.u,&tx_uuid.u)==0)tx_handle=c->val_handle;if(ble_uuid_cmp(&c->uuid.u,&rx_uuid.u)==0)rx_handle=c->val_handle;}if(e->status==BLE_HS_EDONE&&tx_handle&&rx_handle)ble_gattc_disc_all_dscs(connection,rx_handle,service_end,descriptor_cb,nullptr);return 0;}
int service_cb(uint16_t,const ble_gatt_error*e,const ble_gatt_svc*s,void*){if(e->status==0&&s){service_start=s->start_handle;service_end=s->end_handle;}if(e->status==BLE_HS_EDONE&&service_start)ble_gattc_disc_all_chrs(connection,service_start,service_end,characteristic_cb,nullptr);return 0;}
void scan();
int gap(ble_gap_event*e,void*){switch(e->type){case BLE_GAP_EVENT_DISC:{ble_hs_adv_fields f{};if(ble_hs_adv_parse_fields(&f,e->disc.data,e->disc.length_data)==0&&f.name&&std::string((char*)f.name,f.name_len)==cfg.ble_identity){ble_gap_disc_cancel();ble_gap_connect(own_addr_type,&e->disc.addr,30000,nullptr,gap,nullptr);}break;}case BLE_GAP_EVENT_CONNECT:if(e->connect.status==0){connection=e->connect.conn_handle;connected=true;ble_gattc_exchange_mtu(connection,nullptr,nullptr);ble_gattc_disc_svc_by_uuid(connection,&service_uuid.u,service_cb,nullptr);}else scan();break;case BLE_GAP_EVENT_NOTIFY_RX:if(e->notify_rx.attr_handle==rx_handle){size_t n=OS_MBUF_PKTLEN(e->notify_rx.om),old=rx_bytes.size();rx_bytes.resize(old+n);os_mbuf_copydata(e->notify_rx.om,0,n,rx_bytes.data()+old);consume();}break;case BLE_GAP_EVENT_DISCONNECT:connected=false;connection=BLE_HS_CONN_HANDLE_NONE;tx_handle=rx_handle=cccd_handle=0;scan();break;default:break;}return 0;}
void scan(){ble_gap_disc_params p{};p.filter_duplicates=1;p.passive=0;ble_gap_disc(own_addr_type,BLE_HS_FOREVER,&p,gap,nullptr);}
PaymentOutcome exchange(uint8_t type,const std::string&reference,int64_t amount){if(!bluepad_ble_ready())return{PaymentCertainty::NotSent,"","","","BLUEPAD_UNAVAILABLE"};outcome={PaymentCertainty::Unknown,reference,"","","BLUEPAD_TIMEOUT"};while(xSemaphoreTake(response_sem,0)==pdTRUE){};auto bytes=packet(TX_START,type,amount,reference);for(size_t offset=0;offset<bytes.size();offset+=19){size_t n=std::min<size_t>(19,bytes.size()-offset);if(ble_gattc_write_flat(connection,tx_handle,bytes.data()+offset,n,nullptr,nullptr)!=0)return{PaymentCertainty::NotSent,"","","","BLUEPAD_WRITE_FAILED"};}if(xSemaphoreTake(response_sem,pdMS_TO_TICKS(120000))!=pdTRUE)return outcome;return outcome;}
PaymentOutcome lookup(const std::string&reference){if(!bluepad_ble_ready())return{PaymentCertainty::NotSent,"","","","BLUEPAD_UNAVAILABLE"};outcome={PaymentCertainty::Unknown,reference,"","","BLUEPAD_LOOKUP_TIMEOUT"};while(xSemaphoreTake(response_sem,0)==pdTRUE){};auto bytes=lookup_packet(reference);for(size_t offset=0;offset<bytes.size();offset+=19){size_t n=std::min<size_t>(19,bytes.size()-offset);if(ble_gattc_write_flat(connection,tx_handle,bytes.data()+offset,n,nullptr,nullptr)!=0)return{PaymentCertainty::NotSent,"","","","BLUEPAD_LOOKUP_WRITE_FAILED"};}if(xSemaphoreTake(response_sem,pdMS_TO_TICKS(15000))!=pdTRUE)return outcome;return outcome;}
}}
namespace beefiscal::idf{
esp_err_t bluepad_ble_start(const PaymentEndpointBinding&v){if(!v.present||v.transport!="BLE_GATT"||v.ble_identity.empty()||ble_uuid_from_str(&service_uuid,v.service_uuid.c_str())||ble_uuid_from_str(&tx_uuid,v.tx_characteristic_uuid.c_str())||ble_uuid_from_str(&rx_uuid,v.rx_characteristic_uuid.c_str()))return ESP_ERR_INVALID_ARG;cfg=v;if(!response_sem)response_sem=xSemaphoreCreateBinary();configured=response_sem!=nullptr;return configured?ESP_OK:ESP_ERR_NO_MEM;}
void bluepad_ble_on_host_sync(){if(configured&&ble_hs_id_infer_auto(0,&own_addr_type)==0)scan();}
bool bluepad_ble_ready(){return connected&&tx_handle&&rx_handle&&cccd_handle;}
PaymentOutcome bluepad_ble_purchase(const std::string&id,int64_t amount){return exchange(PURCHASE,id,amount);}
PaymentOutcome bluepad_ble_lookup(const std::string&id){return lookup(id);}
PaymentOutcome bluepad_ble_reverse(const std::string&id,int64_t amount){return exchange(VOID_PURCHASE,id,amount);}
}
