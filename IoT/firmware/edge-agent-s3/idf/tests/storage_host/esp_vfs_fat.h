#pragma once
#include <cstdint>
#include "esp_err.h"
#include "sdmmc_cmd.h"
struct esp_vfs_fat_sdmmc_mount_config_t{bool format_if_mount_failed;int max_files;int allocation_unit_size;bool disk_status_check_enable;bool use_one_fat;};
inline int esp_vfs_fat_sdmmc_mount(const char*,void*,void*,void*,sdmmc_card_t**){return ESP_FAIL;}
inline void esp_vfs_fat_sdcard_unmount(const char*,sdmmc_card_t*){}
inline int esp_vfs_fat_info(const char*,uint64_t*,uint64_t*){return ESP_FAIL;}
