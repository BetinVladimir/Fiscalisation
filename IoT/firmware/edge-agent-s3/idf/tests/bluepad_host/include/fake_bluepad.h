#pragma once
#include <cstdint>
#include <cstddef>
#include <vector>
#include <mutex>
#include <condition_variable>
#include <string>
using esp_err_t=int;
constexpr int ESP_OK=0,ESP_FAIL=-1,ESP_ERR_INVALID_ARG=258,ESP_ERR_INVALID_STATE=259,ESP_ERR_NO_MEM=257;
constexpr int BLE_HS_CONN_HANDLE_NONE=65535,BLE_HS_EDONE=14,BLE_HS_FOREVER=2147483647;
constexpr int BLE_GATT_DSC_CLT_CFG_UUID16=0x2902,BLE_GATT_CHR_PROP_WRITE=8,BLE_GATT_CHR_PROP_NOTIFY=16;
constexpr int BLE_GAP_EVENT_DISC=1,BLE_GAP_EVENT_CONNECT=2,BLE_GAP_EVENT_NOTIFY_RX=3,BLE_GAP_EVENT_DISCONNECT=4;
constexpr int BLE_ERR_REM_USER_CONN_TERM=19;
struct ble_uuid_t{uint16_t value{};char text[40]{};};struct ble_uuid_any_t{ble_uuid_t u;};
struct ble_addr_t{uint8_t type{},val[6]{};};
struct os_mbuf{std::vector<uint8_t> bytes;};
#define OS_MBUF_PKTLEN(m) ((m)->bytes.size())
int os_mbuf_copydata(const os_mbuf*,int,int,void*);
struct ble_gap_event{int type{};struct{uint8_t*data{};uint8_t length_data{};ble_addr_t addr;}disc;struct{int status{};uint16_t conn_handle{};}connect;struct{uint16_t conn_handle{},attr_handle{};os_mbuf*om{};}notify_rx;struct{struct{uint16_t conn_handle{};}conn;}disconnect;};
struct ble_gap_disc_params{int filter_duplicates{},passive{};};
struct ble_hs_adv_fields{uint8_t*name{};uint8_t name_len{};};
struct ble_gatt_error{uint16_t status{},att_handle{};};struct ble_gatt_svc{uint16_t start_handle{},end_handle{};ble_uuid_any_t uuid;};
struct ble_gatt_chr{uint16_t def_handle{},val_handle{};uint8_t properties{};ble_uuid_any_t uuid;};struct ble_gatt_dsc{uint16_t handle{};ble_uuid_any_t uuid;};
struct ble_gatt_attr{uint16_t handle{},offset{};os_mbuf*om{};};
using ble_gap_event_fn=int(ble_gap_event*,void*);using ble_gatt_disc_svc_fn=int(uint16_t,const ble_gatt_error*,const ble_gatt_svc*,void*);using ble_gatt_chr_fn=int(uint16_t,const ble_gatt_error*,const ble_gatt_chr*,void*);using ble_gatt_dsc_fn=int(uint16_t,const ble_gatt_error*,uint16_t,const ble_gatt_dsc*,void*);using ble_gatt_attr_fn=int(uint16_t,const ble_gatt_error*,ble_gatt_attr*,void*);using ble_gatt_mtu_fn=int(uint16_t,const ble_gatt_error*,uint16_t,void*);
int ble_uuid_from_str(ble_uuid_any_t*,const char*);int ble_uuid_cmp(const ble_uuid_t*,const ble_uuid_t*);uint16_t ble_uuid_u16(const ble_uuid_t*);
int ble_hs_id_infer_auto(int,uint8_t*);int ble_hs_adv_parse_fields(ble_hs_adv_fields*,const uint8_t*,uint8_t);
int ble_gap_disc(uint8_t,int,const ble_gap_disc_params*,ble_gap_event_fn*,void*);int ble_gap_disc_cancel();int ble_gap_connect(uint8_t,const ble_addr_t*,int,const void*,ble_gap_event_fn*,void*);int ble_gap_terminate(uint16_t,uint8_t);
int ble_gattc_exchange_mtu(uint16_t,ble_gatt_mtu_fn*,void*);int ble_gattc_disc_svc_by_uuid(uint16_t,const ble_uuid_t*,ble_gatt_disc_svc_fn*,void*);int ble_gattc_disc_all_chrs(uint16_t,uint16_t,uint16_t,ble_gatt_chr_fn*,void*);int ble_gattc_disc_all_dscs(uint16_t,uint16_t,uint16_t,ble_gatt_dsc_fn*,void*);int ble_gattc_write_flat(uint16_t,uint16_t,const void*,uint16_t,ble_gatt_attr_fn*,void*);int ble_gattc_read(uint16_t,uint16_t,ble_gatt_attr_fn*,void*);
int64_t esp_timer_get_time();
struct FakeSemaphore{std::mutex mutex;std::condition_variable cv;bool given{};};using SemaphoreHandle_t=FakeSemaphore*;using TickType_t=uint32_t;
constexpr int pdTRUE=1,pdFALSE=0;constexpr uint32_t portMAX_DELAY=0xffffffff;
#define pdMS_TO_TICKS(v) (v)
SemaphoreHandle_t xSemaphoreCreateBinary();int xSemaphoreTake(SemaphoreHandle_t,uint32_t);int xSemaphoreGive(SemaphoreHandle_t);
