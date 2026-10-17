#pragma once
using gpio_num_t=int;struct sdmmc_host_t{};struct sdmmc_slot_config_t{int width,clk,cmd,d0;};
#define SDMMC_HOST_DEFAULT() sdmmc_host_t{}
#define SDMMC_SLOT_CONFIG_DEFAULT() sdmmc_slot_config_t{}
