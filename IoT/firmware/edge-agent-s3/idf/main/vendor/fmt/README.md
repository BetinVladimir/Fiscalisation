# Pinned portable float conversion dependency

fmt 11.0.2 (`FMT_VERSION 110002`), MIT license included in each header.
Source: https://github.com/fmtlib/fmt/tree/11.0.2/include/fmt
Copied byte-for-byte from installed ReactNativeDependencies headers; three files
are sufficient for FMT_HEADER_ONLY. Only pinned internal Dragonbox API is used.
No dynamic allocation or locale-dependent printf occurs in float conversion.

SHA256:
- `base.h`: `895fd797b2c203e3661cdbeb7ee4f60a637c927be1b4a4728b9aa6400e238d90`
- `format-inl.h`: `cb9ddc93e2daa1fecddd43b43a924dac8fa7300672285f36671d9457f0ffc3a8`
- `format.h`: `b2119dab01c5f640651f92f5e37cc0e8bb7ddcca50b2f6a7a67b662ffbebee00`
