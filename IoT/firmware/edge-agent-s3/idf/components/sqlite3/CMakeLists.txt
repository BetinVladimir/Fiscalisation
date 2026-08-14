idf_component_register(SRCS "sqlite3.c" "esp_idf_posix_compat.c" INCLUDE_DIRS "include" "." REQUIRES freertos)
target_compile_definitions(${COMPONENT_LIB} PUBLIC
  SQLITE_THREADSAFE=1
  SQLITE_DEFAULT_MEMSTATUS=0
  SQLITE_OMIT_LOAD_EXTENSION=1
  SQLITE_TEMP_STORE=2
  SQLITE_DEFAULT_WAL_SYNCHRONOUS=1
)
target_compile_options(${COMPONENT_LIB} PRIVATE -Wno-implicit-fallthrough -Wno-unused-function)
