idf_component_register(
  SRCS
    "app_main.cpp"
    "edge_crypto.cpp"
    "mqtt_runtime.cpp"
    "ble_runtime.cpp"
    "usb_cdc_runtime.cpp"
    "durable_storage.cpp"
    "provisioned_binding.cpp"
    "runtime_config.cpp"
    "intent_processor.cpp"
    "command_queue.cpp"
    "canonical_cbor.cpp"
    "device_identity.cpp"
    "sync_runtime.cpp"
    "uart_runtime.cpp"
    "profile_executor.cpp"
    "profile_orchestrator.cpp"
    "receipt_saga.cpp"
    "bluepad_ble_central.cpp"
    "../../../../protocol-abstraction/src/CommandPayload.cpp"
    "../../../../protocol-abstraction/src/FrameCodec.cpp"
    "../../../../protocol-abstraction/src/AllCommands.cpp"
  INCLUDE_DIRS "." "../../../../protocol-abstraction/include"
  REQUIRES esp_event esp_netif esp_wifi mqtt nvs_flash mbedtls json bt usb usb_host_cdc_acm sqlite3 fatfs sdmmc esp_driver_uart
)
target_compile_features(${COMPONENT_LIB} PRIVATE cxx_std_17)
