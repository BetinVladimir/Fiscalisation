#pragma once
#include <cstddef>
namespace beefiscal::idf {
// UTF-8 bytes, shared with the root issuer wire contract. Never truncate identity.
inline constexpr std::size_t kAuthorityActorMaxBytes = 256;
}
