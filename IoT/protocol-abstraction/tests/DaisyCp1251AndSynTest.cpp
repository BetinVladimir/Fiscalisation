// VG-17.3: DaisyTaxGroupUsesCyrillicCp1251 and DaisySynEvery100ms
// Verifies that:
//   1. buildDaisySaleItem encodes Latin 'A'..'H' as CP1251 Cyrillic 0xC0..0xC7 — never 0x41..0x48.
//   2. DAISY_SYN == 0x16; SYN bytes are not accepted as valid frames by DaisyCodec::decode.
#include "CommandPayload.hpp"
#include "FrameCodec.hpp"
#include "DaisyProtocol.h"
#include <iostream>
#include <vector>
#include <string>

using namespace bee;

static int failures = 0;
static void check(bool ok, const char* name) {
    if (!ok) { std::cerr << "FAIL " << name << '\n'; ++failures; }
}

int main() {
    // --- DaisyTaxGroupUsesCyrillicCp1251 ---
    // buildDaisySaleItem must encode Latin 'A' (0x41) as CP1251 Cyrillic А (0xC0).
    {
        SaleItemPayload item{};
        item.name = "Coffee";
        item.unitPrice = "1.00";
        item.quantity = "1.000";
        std::vector<uint8_t> payload;
        std::string error;

        // Group A → must become 0xC0 (CP1251 А), not 0x41 (Latin A)
        item.taxGroup = 'A';
        bool built = buildDaisySaleItem(item, payload, error);
        check(built, "DaisyTaxGroupUsesCyrillicCp1251 build_A");
        // payload layout: name + '\t' + <tax_byte> + price + ...
        // "Coffee" = 6 bytes, '\t' = 1 → tax byte at index 7
        check(built && payload.size() > 7, "DaisyTaxGroupUsesCyrillicCp1251 payload_not_empty_A");
        check(built && payload.size() > 7 && payload[7] == 0xC0,
              "DaisyTaxGroupUsesCyrillicCp1251 taxA_is_0xC0");
        check(built && payload.size() > 7 && payload[7] != 0x41,
              "DaisyTaxGroupUsesCyrillicCp1251 taxA_not_latin_0x41");

        // Group H (highest valid) → must become 0xC7 (CP1251 З)
        item.taxGroup = 'H';
        bool builtH = buildDaisySaleItem(item, payload, error);
        check(builtH && payload.size() > 7 && payload[7] == 0xC7,
              "DaisyTaxGroupUsesCyrillicCp1251 taxH_is_0xC7");
        check(builtH && payload.size() > 7 && payload[7] != 0x48,
              "DaisyTaxGroupUsesCyrillicCp1251 taxH_not_latin_0x48");

        // A Latin letter outside 'A'..'H' must be rejected (no tax group I)
        item.taxGroup = 'I';
        check(!buildDaisySaleItem(item, payload, error),
              "DaisyTaxGroupUsesCyrillicCp1251 taxI_rejected");

        if (!failures) std::cout << "DaisyTaxGroupUsesCyrillicCp1251=pass\n";
    }

    // --- DaisySynEvery100ms ---
    // The Daisy protocol spec (v2.0-4 §1): device sends SYN (0x16) every 100 ms
    // while processing.  The PC must wait for a real response.
    // Verify: DAISY_SYN == 0x16 and that DaisyCodec::decode treats a SYN-only
    // byte sequence as an error (not a valid response frame).
    {
        static_assert(DAISY_SYN == 0x16,
                      "DAISY_SYN must be 0x16 per Daisy v2.0-4 §1 (non-packed codes)");
        static_assert(DAISY_TAX_BASE == 0xC0,
                      "DAISY_TAX_BASE must be 0xC0 (CP1251 Cyrillic А)");

        check(DAISY_SYN == 0x16, "DaisySynEvery100ms DAISY_SYN_is_0x16");

        ParsedFrame out{};
        std::string err;

        // A lone SYN byte must not decode as a valid frame
        std::vector<uint8_t> synByte{0x16};
        check(!DaisyCodec::decode(synByte, out, err),
              "DaisySynEvery100ms single_SYN_not_a_valid_frame");

        // A buffer filled with SYN bytes must also be rejected
        std::vector<uint8_t> manySyn(32, 0x16);
        check(!DaisyCodec::decode(manySyn, out, err),
              "DaisySynEvery100ms many_SYN_not_a_valid_frame");

        if (!failures) std::cout << "DaisySynEvery100ms=pass\n";
    }

    if (failures) { std::cerr << failures << " failures\n"; return 1; }
    std::cout << "DaisyCp1251AndSynTest: OK\n";
    return 0;
}
