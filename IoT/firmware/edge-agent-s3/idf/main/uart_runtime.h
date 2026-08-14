#pragma once
#include <cstddef>
#include <cstdint>
#include "esp_err.h"
#include "provisioned_binding.h"
namespace beefiscal::idf{
esp_err_t uart_runtime_start(const FiscalEndpointBinding&);
bool uart_runtime_ready();int uart_runtime_write(const uint8_t*,size_t,uint32_t);
int uart_runtime_read(uint8_t*,size_t,uint32_t);
}
