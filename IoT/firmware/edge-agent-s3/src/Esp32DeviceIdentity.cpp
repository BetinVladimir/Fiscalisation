#include "Esp32DeviceIdentity.h"

#include <esp_system.h>
#include <mbedtls/ecdsa.h>
#include <mbedtls/sha256.h>
#include <string.h>

namespace beefiscal::edge {
namespace {

constexpr char kNvsNamespace[] = "bf_identity";
constexpr char kPrivateKeyName[] = "p256_scalar";
constexpr size_t kScalarSize = 32;

int espRandom(void*, unsigned char* output, size_t length) {
    esp_fill_random(output, length);
    return 0;
}

String base64Url(const uint8_t* input, size_t length) {
    static constexpr char alphabet[] =
        "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";
    String output;
    output.reserve(((length * 4) + 2) / 3);
    for (size_t offset = 0; offset < length; offset += 3) {
        const size_t remaining = length - offset;
        const uint32_t value = (static_cast<uint32_t>(input[offset]) << 16) |
            (remaining > 1 ? static_cast<uint32_t>(input[offset + 1]) << 8 : 0) |
            (remaining > 2 ? input[offset + 2] : 0);
        output += alphabet[(value >> 18) & 0x3f];
        output += alphabet[(value >> 12) & 0x3f];
        if (remaining > 1) output += alphabet[(value >> 6) & 0x3f];
        if (remaining > 2) output += alphabet[value & 0x3f];
    }
    return output;
}

bool writeFixed(const mbedtls_mpi& value, uint8_t output[kScalarSize]) {
    return mbedtls_mpi_write_binary(&value, output, kScalarSize) == 0;
}

} // namespace

Esp32DeviceIdentity::Esp32DeviceIdentity() {
    mbedtls_ecp_keypair_init(&key_);
}

Esp32DeviceIdentity::~Esp32DeviceIdentity() {
    mbedtls_ecp_keypair_free(&key_);
    if (preferencesOpen_) preferences_.end();
}

bool Esp32DeviceIdentity::begin() {
    if (ready_) return true;
    if (!preferences_.begin(kNvsNamespace, false)) return false;
    preferencesOpen_ = true;

    const size_t persistedLength = preferences_.getBytesLength(kPrivateKeyName);
    if (persistedLength == 0) {
        mbedtls_ecp_keypair_free(&key_);
        mbedtls_ecp_keypair_init(&key_);
        if (!generate() || !selfTest() || !persist()) return false;
        generatedOnThisBoot_ = true;
    } else if (persistedLength != kScalarSize || !load() || !selfTest()) {
        // Never silently rotate a corrupted identity: backend registration and
        // audit continuity are bound to the original public key.
        return false;
    }

    ready_ = true;
    return true;
}

bool Esp32DeviceIdentity::load() {
    if (preferences_.getBytesLength(kPrivateKeyName) != kScalarSize) return false;
    uint8_t scalar[kScalarSize]{};
    const size_t read = preferences_.getBytes(kPrivateKeyName, scalar, sizeof(scalar));
    if (read != sizeof(scalar)) return false;

    int result = mbedtls_ecp_group_load(&key_.grp, MBEDTLS_ECP_DP_SECP256R1);
    if (result == 0) result = mbedtls_mpi_read_binary(&key_.d, scalar, sizeof(scalar));
    memset(scalar, 0, sizeof(scalar));
    if (result != 0 || mbedtls_mpi_cmp_int(&key_.d, 1) < 0 ||
        mbedtls_mpi_cmp_mpi(&key_.d, &key_.grp.N) >= 0) {
        return false;
    }
    result = mbedtls_ecp_mul(&key_.grp, &key_.Q, &key_.d, &key_.grp.G,
                             espRandom, nullptr);
    return result == 0 && mbedtls_ecp_check_pubkey(&key_.grp, &key_.Q) == 0;
}

bool Esp32DeviceIdentity::generate() {
    return mbedtls_ecp_gen_key(MBEDTLS_ECP_DP_SECP256R1, &key_, espRandom,
                               nullptr) == 0;
}

bool Esp32DeviceIdentity::persist() {
    uint8_t scalar[kScalarSize]{};
    if (!writeFixed(key_.d, scalar)) return false;
    const size_t written = preferences_.putBytes(kPrivateKeyName, scalar, sizeof(scalar));
    memset(scalar, 0, sizeof(scalar));
    return written == kScalarSize;
}

bool Esp32DeviceIdentity::sign(const uint8_t* message, size_t messageLength,
                               uint8_t signature[64]) {
    if (!ready_ && !preferencesOpen_) return false;
    if ((message == nullptr && messageLength != 0) || signature == nullptr) return false;
    uint8_t digest[32]{};
    if (mbedtls_sha256_ret(message, messageLength, digest, 0) != 0) return false;

    mbedtls_mpi r;
    mbedtls_mpi s;
    mbedtls_mpi_init(&r);
    mbedtls_mpi_init(&s);
    const int result = mbedtls_ecdsa_sign(&key_.grp, &r, &s, &key_.d,
                                          digest, sizeof(digest), espRandom, nullptr);
    bool ok = result == 0 && writeFixed(r, signature) && writeFixed(s, signature + 32);
    mbedtls_mpi_free(&r);
    mbedtls_mpi_free(&s);
    memset(digest, 0, sizeof(digest));
    return ok;
}

bool Esp32DeviceIdentity::selfTest() {
    static constexpr uint8_t challenge[] = "beefiscal-device-identity-self-test-v1";
    uint8_t signature[64]{};
    if (!sign(challenge, sizeof(challenge) - 1, signature)) return false;
    uint8_t digest[32]{};
    if (mbedtls_sha256_ret(challenge, sizeof(challenge) - 1, digest, 0) != 0) return false;
    mbedtls_mpi r;
    mbedtls_mpi s;
    mbedtls_mpi_init(&r);
    mbedtls_mpi_init(&s);
    int result = mbedtls_mpi_read_binary(&r, signature, 32);
    if (result == 0) result = mbedtls_mpi_read_binary(&s, signature + 32, 32);
    if (result == 0) {
        result = mbedtls_ecdsa_verify(&key_.grp, digest, sizeof(digest),
                                      &key_.Q, &r, &s);
    }
    mbedtls_mpi_free(&r);
    mbedtls_mpi_free(&s);
    memset(signature, 0, sizeof(signature));
    memset(digest, 0, sizeof(digest));
    return result == 0;
}

String Esp32DeviceIdentity::publicJwk() const {
    if (!ready_) return String();
    uint8_t x[32]{};
    uint8_t y[32]{};
    if (!writeFixed(key_.Q.X, x) || !writeFixed(key_.Q.Y, y)) return String();
    return String("{\"crv\":\"P-256\",\"kty\":\"EC\",\"x\":\"") +
        base64Url(x, sizeof(x)) + "\",\"y\":\"" + base64Url(y, sizeof(y)) + "\"}";
}

String Esp32DeviceIdentity::publicKeyThumbprint() const {
    const String jwk = publicJwk();
    if (jwk.isEmpty()) return String();
    uint8_t digest[32]{};
    if (mbedtls_sha256_ret(reinterpret_cast<const uint8_t*>(jwk.c_str()),
                           jwk.length(), digest, 0) != 0) return String();
    return base64Url(digest, sizeof(digest));
}

} // namespace beefiscal::edge
