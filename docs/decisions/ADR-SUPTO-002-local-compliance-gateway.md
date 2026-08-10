# ADR-SUPTO-002: Local Compliance Gateway

Online and offline POS use the same intent contract. A local gateway, not the UI, freezes route/authority, applies the country profile, persists before device I/O and performs lookup-only recovery after ambiguity. Direct POS fiscal envelopes are forbidden in the target profile.
