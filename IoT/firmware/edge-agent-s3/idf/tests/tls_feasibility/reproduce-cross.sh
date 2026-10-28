#!/bin/sh
set -eu
cd "$(dirname "$0")"
test "$(git -C mbedtls-3.6.7 rev-parse HEAD)" = 068ff080b369adfac81509f9b57b2afabaf82dc5
test "$(git -C mbedtls-3.6.7/framework rev-parse HEAD)" = dde0c4a0e448a0552f18817dcea633bb851fd288
git -C mbedtls-3.6.7 diff --quiet HEAD --
git -C mbedtls-3.6.7/framework diff --quiet HEAD --
task_ndk=${ANDROID_NDK_HOME:-/Users/freelancer/Library/Android/sdk/ndk/27.1.12297006}
cmake -S mbedtls-3.6.7 -B build-android -DENABLE_TESTING=OFF -DENABLE_PROGRAMS=OFF -DCMAKE_TOOLCHAIN_FILE="$task_ndk/build/cmake/android.toolchain.cmake" -DANDROID_ABI=arm64-v8a -DANDROID_PLATFORM=android-24
cmake --build build-android -j 8
"$task_ndk/toolchains/llvm/prebuilt/darwin-x86_64/bin/aarch64-linux-android24-clang" -O2 -I mbedtls-3.6.7/include -c engine.c -o engine-android-arm64.o
"$task_ndk/toolchains/llvm/prebuilt/darwin-x86_64/bin/aarch64-linux-android24-clang" -shared engine-android-arm64.o build-android/library/libmbedtls.a build-android/library/libmbedx509.a build-android/library/libmbedcrypto.a -o libchannel-android-arm64.so
cmake -S mbedtls-3.6.7 -B build-ios-device -DENABLE_TESTING=OFF -DENABLE_PROGRAMS=OFF -DCMAKE_SYSTEM_NAME=iOS -DCMAKE_OSX_SYSROOT=iphoneos -DCMAKE_OSX_ARCHITECTURES=arm64 -DCMAKE_OSX_DEPLOYMENT_TARGET=15.1
cmake --build build-ios-device -j 8
xcrun --sdk iphoneos clang -target arm64-apple-ios15.1 -O2 -I mbedtls-3.6.7/include -c engine.c -o engine-ios-device-arm64.o
xcrun --sdk iphoneos libtool -static engine-ios-device-arm64.o build-ios-device/library/libmbedtls.a build-ios-device/library/libmbedx509.a build-ios-device/library/libmbedcrypto.a -o libchannel-ios-device-arm64.a
task_sim_sdk=$(xcrun --sdk iphonesimulator --show-sdk-path)
cmake -S mbedtls-3.6.7 -B build-ios -DENABLE_TESTING=OFF -DENABLE_PROGRAMS=OFF -DCMAKE_SYSTEM_NAME=iOS -DCMAKE_OSX_SYSROOT="$task_sim_sdk" -DCMAKE_OSX_ARCHITECTURES=arm64 -DCMAKE_OSX_DEPLOYMENT_TARGET=15.1
cmake --build build-ios -j 8
xcrun --sdk iphonesimulator clang -target arm64-apple-ios15.1-simulator -O2 -I mbedtls-3.6.7/include -c engine.c -o engine-ios-simulator-arm64.o
xcrun --sdk iphonesimulator clang -target arm64-apple-ios15.1-simulator -dynamiclib engine-ios-simulator-arm64.o build-ios/library/libmbedtls.a build-ios/library/libmbedx509.a build-ios/library/libmbedcrypto.a -o libchannel-ios-simulator-arm64.dylib
./emsdk/upstream/emscripten/emcmake cmake -S mbedtls-3.6.7 -B build-wasm -DENABLE_TESTING=OFF -DENABLE_PROGRAMS=OFF
cmake --build build-wasm -j 8
./emsdk/upstream/emscripten/emcc -O2 -I mbedtls-3.6.7/include main.c build-wasm/library/libmbedtls.a build-wasm/library/libmbedx509.a build-wasm/library/libmbedcrypto.a -sASSERTIONS=2 -sSTACK_SIZE=262144 -sALLOW_MEMORY_GROWTH=1 -o c-fixture-wasm.js
./emsdk/node/22.16.0_64bit/bin/node c-fixture-wasm.js
./emsdk/node/22.16.0_64bit/bin/node c-fixture-wasm.js wrong.spike.test
./emsdk/node/22.16.0_64bit/bin/node c-fixture-wasm.js expired
file libchannel-android-arm64.so libchannel-ios-device-arm64.a libchannel-ios-simulator-arm64.dylib
