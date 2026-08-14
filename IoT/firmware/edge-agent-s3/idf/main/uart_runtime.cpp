#include "uart_runtime.h"
#include "driver/uart.h"
#include <atomic>
namespace beefiscal::idf{namespace{constexpr uart_port_t port=UART_NUM_1;std::atomic_bool active{false};}
esp_err_t uart_runtime_start(const FiscalEndpointBinding&b){if(b.uart_tx_pin<0||b.uart_rx_pin<0||b.uart_baud<1200)return ESP_ERR_INVALID_ARG;uart_config_t c{};c.baud_rate=b.uart_baud;c.data_bits=b.uart_data_bits==7?UART_DATA_7_BITS:UART_DATA_8_BITS;c.parity=b.uart_parity=='E'?UART_PARITY_EVEN:b.uart_parity=='O'?UART_PARITY_ODD:UART_PARITY_DISABLE;c.stop_bits=b.uart_stop_bits==2?UART_STOP_BITS_2:UART_STOP_BITS_1;c.flow_ctrl=UART_HW_FLOWCTRL_DISABLE;c.source_clk=UART_SCLK_DEFAULT;esp_err_t e=uart_driver_install(port,4096,4096,0,nullptr,0);if(e==ESP_OK)e=uart_param_config(port,&c);if(e==ESP_OK)e=uart_set_pin(port,b.uart_tx_pin,b.uart_rx_pin,UART_PIN_NO_CHANGE,UART_PIN_NO_CHANGE);active=e==ESP_OK;return e;}
bool uart_runtime_ready(){return active.load();}
int uart_runtime_write(const uint8_t*p,size_t n,uint32_t timeout){if(!active||!p||!n)return-1;int w=uart_write_bytes(port,p,n);if(w!=(int)n||uart_wait_tx_done(port,pdMS_TO_TICKS(timeout))!=ESP_OK)return-1;return w;}
int uart_runtime_read(uint8_t*p,size_t n,uint32_t timeout){if(!active||!p||!n)return-1;return uart_read_bytes(port,p,n,pdMS_TO_TICKS(timeout));}
}
