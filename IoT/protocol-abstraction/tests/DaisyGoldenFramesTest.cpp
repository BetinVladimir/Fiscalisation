#include "FrameCodec.hpp"
#include <cstdint>
#include <fstream>
#include <iostream>
#include <sstream>
#include <stdexcept>
#include <string>
#include <vector>

using Bytes = std::vector<uint8_t>;

// ---------------------------------------------------------------------------
// Minimal JSON array parser — reads the flat fixture without external libs.
// Only handles the keys used by daisy_golden_frames.json.
// ---------------------------------------------------------------------------
struct Frame {
    int cmd{};
    int seq{};
    std::string direction;
    std::string data_hex;
    std::string frame_hex;
    bool bcc_verified{true};
    bool negative{false};
};

static std::string unquote(const std::string& s) {
    if (s.size() >= 2 && s.front() == '"' && s.back() == '"')
        return s.substr(1, s.size() - 2);
    return s;
}

static uint8_t hex_nybble(char c) {
    if (c >= '0' && c <= '9') return uint8_t(c - '0');
    if (c >= 'a' && c <= 'f') return uint8_t(c - 'a' + 10);
    if (c >= 'A' && c <= 'F') return uint8_t(c - 'A' + 10);
    throw std::runtime_error(std::string("bad hex char: ") + c);
}

static Bytes from_hex(const std::string& h) {
    Bytes v;
    for (size_t i = 0; i + 1 < h.size(); i += 2)
        v.push_back(uint8_t((hex_nybble(h[i]) << 4) | hex_nybble(h[i+1])));
    return v;
}

static std::vector<Frame> load_frames(const std::string& path) {
    std::ifstream f(path);
    if (!f) throw std::runtime_error("cannot open: " + path);
    std::string content((std::istreambuf_iterator<char>(f)),
                         std::istreambuf_iterator<char>());
    std::vector<Frame> frames;
    Frame cur;
    bool in_object = false;
    std::istringstream ss(content);
    std::string line;
    while (std::getline(ss, line)) {
        auto trim = [](std::string s) {
            size_t a = s.find_first_not_of(" \t\r\n");
            size_t b = s.find_last_not_of(" \t\r\n,");
            return (a == std::string::npos) ? "" : s.substr(a, b - a + 1);
        };
        std::string t = trim(line);
        if (t == "{") { cur = Frame{}; in_object = true; continue; }
        if ((t == "}" || t == "},") && in_object) {
            frames.push_back(cur);
            in_object = false;
            continue;
        }
        if (!in_object) continue;
        auto colon = t.find(':');
        if (colon == std::string::npos) continue;
        std::string key = unquote(trim(t.substr(0, colon)));
        std::string val = trim(t.substr(colon + 1));
        if (key == "cmd")        cur.cmd = std::stoi(val);
        else if (key == "seq")   cur.seq = std::stoi(val);
        else if (key == "direction")  cur.direction = unquote(val);
        else if (key == "data_hex")   cur.data_hex  = unquote(val);
        else if (key == "frame_hex")  cur.frame_hex = unquote(val);
        else if (key == "bcc_verified") cur.bcc_verified = (val == "true");
        else if (key == "negative")   cur.negative  = (val == "true");
    }
    return frames;
}

// ---------------------------------------------------------------------------

static int failures = 0;
static void check(bool ok, const std::string& name) {
    if (!ok) { std::cerr << "FAIL " << name << '\n'; ++failures; }
}

int main(int argc, char* argv[]) {
    // Fixture path: first arg or sibling of test binary (CI convention).
    std::string fixture_path =
        (argc > 1) ? argv[1]
                   : "docs/MVP1/fixtures/daisy_golden_frames.json";

    std::vector<Frame> frames;
    try {
        frames = load_frames(fixture_path);
    } catch (const std::exception& e) {
        std::cerr << "ERROR loading fixture: " << e.what() << '\n';
        return 1;
    }
    if (frames.empty()) {
        std::cerr << "ERROR: fixture loaded 0 frames\n";
        return 1;
    }

    int positive_count = 0;
    int negative_count = 0;

    for (const auto& fr : frames) {
        const Bytes expected = from_hex(fr.frame_hex);
        const Bytes data     = from_hex(fr.data_hex);
        const std::string tag =
            "cmd=" + std::to_string(fr.cmd) +
            " seq=0x" + [&]{ std::ostringstream s; s << std::hex << fr.seq; return s.str(); }() +
            " dir=" + fr.direction;

        if (fr.negative) {
            // Negative vector: decoder must reject it.
            bee::ParsedFrame out; std::string err;
            bool decoded = bee::DaisyCodec::decode(expected, out, err);
            check(!decoded, tag + " negative rejected");
            ++negative_count;
            continue;
        }

        if (fr.direction == "request") {
            // Encoder: encode(seq, cmd, data) must match frame_hex byte-for-byte.
            Bytes encoded = bee::DaisyCodec::encode(
                uint8_t(fr.seq), uint8_t(fr.cmd), data);
            check(encoded == expected, tag + " encode matches fixture");
            ++positive_count;
        } else if (fr.direction == "answer") {
            // Decoder: decode(frame_hex) must recover seq/cmd/data fields.
            bee::ParsedFrame out; std::string err;
            bool ok = bee::DaisyCodec::decode(expected, out, err);
            check(ok,                             tag + " decode ok");
            check(out.command == uint16_t(fr.cmd),  tag + " command");
            check(out.sequence == uint8_t(fr.seq),  tag + " sequence");
            check(out.data == data,                 tag + " data");
            ++positive_count;
        }
    }

    if (positive_count == 0) {
        std::cerr << "ERROR: no positive frames tested\n";
        return 1;
    }
    if (negative_count == 0) {
        std::cerr << "ERROR: no negative frames tested\n";
        return 1;
    }

    if (failures) {
        std::cerr << failures << " failures\n";
        return 1;
    }
    std::cout << "daisy_golden_frames_positive=" << positive_count << '\n';
    std::cout << "daisy_golden_frames_negative=" << negative_count << '\n';
    std::cout << "daisy_golden_frames_test: OK\n";
    return 0;
}
