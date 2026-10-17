#pragma once
#include <algorithm>
#include <cstddef>
#include <cstdint>
namespace beefiscal::idf {
constexpr size_t kBleFrameHeader=57;
constexpr uint16_t kBleMinimumMtu=61; // ATT3 + unchanged BFF1 header57 + >=1 body byte.
constexpr size_t ble_att_value_capacity(uint16_t mtu){return mtu<3?0:std::min<size_t>(mtu-3,512);}
constexpr size_t ble_frame_body_capacity(uint16_t mtu){return mtu<kBleMinimumMtu?0:std::min<size_t>(128,ble_att_value_capacity(mtu)-kBleFrameHeader);}
constexpr size_t kBleScanNameMaximum=29; // legacy scan response31 - AD length/type2.
}
