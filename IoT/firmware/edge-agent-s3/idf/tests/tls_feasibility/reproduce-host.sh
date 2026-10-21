#!/bin/sh
set -eu
cd "$(dirname "$0")"
test "$(git -C mbedtls-3.6.7 rev-parse HEAD)" = 068ff080b369adfac81509f9b57b2afabaf82dc5
test "$(git -C mbedtls-3.6.7/framework rev-parse HEAD)" = dde0c4a0e448a0552f18817dcea633bb851fd288
git -C mbedtls-3.6.7 diff --quiet HEAD --
git -C mbedtls-3.6.7/framework diff --quiet HEAD --
go run gen.go
python3 make_assets.py
cmake -S mbedtls-3.6.7 -B build-host -DENABLE_TESTING=OFF -DENABLE_PROGRAMS=OFF
cmake --build build-host -j 8
cc -O2 -I mbedtls-3.6.7/include main.c -o c-fixture build-host/library/libmbedtls.a build-host/library/libmbedx509.a build-host/library/libmbedcrypto.a
./c-fixture
./c-fixture wrong.spike.test
./c-fixture client-only
./c-fixture expired
go run interop.go
cc -O2 -I mbedtls-3.6.7/include peer_client.c -o c-peer-client build-host/library/libmbedtls.a build-host/library/libmbedx509.a build-host/library/libmbedcrypto.a
go run peer_server.go
python3 counterfactual.py
