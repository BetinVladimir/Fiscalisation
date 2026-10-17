#pragma once
#include <cstdint>
#include <cstddef>
#include <vector>
using esp_err_t=int;
constexpr int ESP_OK=0,ESP_FAIL=-1,ESP_ERR_INVALID_ARG=1,ESP_ERR_INVALID_STATE=2,ESP_ERR_INVALID_SIZE=3,ESP_ERR_INVALID_CRC=4,ESP_ERR_NO_MEM=5,ESP_ERR_TIMEOUT=6;
constexpr int BLE_HS_CONN_HANDLE_NONE=65535,BLE_HS_EINVAL=7,BLE_GATT_ACCESS_OP_WRITE_CHR=1,BLE_ATT_ERR_WRITE_NOT_PERMITTED=2,BLE_ATT_ERR_INVALID_ATTR_VALUE_LEN=3,BLE_ATT_ERR_INSUFFICIENT_RES=4,BLE_ATT_ERR_UNLIKELY=5;
constexpr int BLE_GATT_CHR_F_WRITE=1,BLE_GATT_CHR_F_WRITE_NO_RSP=2,BLE_GATT_CHR_F_NOTIFY=4,BLE_GATT_SVC_TYPE_PRIMARY=1,BLE_GAP_EVENT_CONNECT=1,BLE_GAP_EVENT_DISCONNECT=2,BLE_HS_ADV_F_DISC_GEN=1,BLE_HS_ADV_F_BREDR_UNSUP=2,BLE_GAP_CONN_MODE_UND=1,BLE_GAP_DISC_MODE_GEN=1,BLE_HS_FOREVER=-1;
struct ble_uuid_t{};struct ble_uuid128_t{ble_uuid_t u;uint8_t value[16];};
#define BLE_UUID128_INIT(...) {{},{__VA_ARGS__}}
struct os_mbuf{std::vector<uint8_t> bytes;};
#define OS_MBUF_PKTLEN(om) ((om)->bytes.size())
struct ble_gatt_access_ctxt{int op;os_mbuf*om;};
struct ble_gatt_chr_def{const ble_uuid_t*uuid;int(*access_cb)(uint16_t,uint16_t,ble_gatt_access_ctxt*,void*);void*arg;void*descriptors;int flags;int min_key_size;uint16_t*val_handle;void*cpfd;};
struct ble_gatt_svc_def{int type;const ble_uuid_t*uuid;void*includes;const ble_gatt_chr_def*characteristics;};
struct ble_gap_event{int type;struct{int status;uint16_t conn_handle;}connect;};
struct ble_hs_adv_fields{int flags{};uint8_t*name{};uint8_t name_len{};int name_is_complete{};ble_uuid128_t*uuids128{};int num_uuids128{};int uuids128_is_complete{};};
struct ble_gap_adv_params{int conn_mode{};int disc_mode{};};
struct ble_hs_cfg_t{void(*sync_cb)();};extern ble_hs_cfg_t ble_hs_cfg;
int ble_hs_mbuf_to_flat(os_mbuf*,void*,size_t,uint16_t*);os_mbuf*ble_hs_mbuf_from_flat(const void*,size_t);void os_mbuf_free_chain(os_mbuf*);
int ble_gatts_notify_custom(uint16_t,uint16_t,os_mbuf*);uint16_t ble_att_mtu(uint16_t);
int ble_gap_adv_set_fields(const ble_hs_adv_fields*);int ble_gap_adv_rsp_set_fields(const ble_hs_adv_fields*);
int ble_gap_adv_start(uint8_t,void*,int,const ble_gap_adv_params*,int(*)(ble_gap_event*,void*),void*);int ble_gap_adv_stop();
int ble_hs_util_ensure_addr(int);int ble_hs_id_infer_auto(int,uint8_t*);
esp_err_t esp_nimble_hci_init();esp_err_t nimble_port_init();void nimble_port_run();void nimble_port_freertos_deinit();void nimble_port_freertos_init(void(*)(void*));
void ble_svc_gap_init();void ble_svc_gatt_init();int ble_svc_gap_device_name_set(const char*);int ble_gatts_count_cfg(const ble_gatt_svc_def*);int ble_gatts_add_svcs(const ble_gatt_svc_def*);
int mbedtls_sha256(const unsigned char*,size_t,unsigned char*,int);
using SemaphoreHandle_t=bool*;SemaphoreHandle_t xSemaphoreCreateBinary();int xSemaphoreGive(SemaphoreHandle_t);int xSemaphoreTake(SemaphoreHandle_t,int);
constexpr int pdTRUE=1;
#define pdMS_TO_TICKS(ms) (ms)
