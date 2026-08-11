# Datecs Pay protocol module

`DatecsPay` implements Datecs Pay Pinpad Commands v3 framing over an Arduino
`Stream`. The stream may be UART, USB, or a BLE GATT stream adapter. Financial
commands are serialized: only one command may be in flight, while unsolicited
`0x0B`, `0x0E`, and `0x0F` events are dispatched during response waiting.

## BLE transport contract

For BlueCash/BluePad BLE, use the profile constants in `DatecsPayBleProfile.h`.
The adapter must:

1. connect using LE and discover a service whose UUID starts with the vendor
   service prefix;
2. require POWER, WAKE, READ, and WRITE characteristics;
3. enable notifications for READ and POWER;
4. read POWER; if it is `0x30`, write the peer device name to WAKE and wait for
   POWER=`0x31`;
5. expose READ notifications as ordered `Stream::read()` bytes;
6. split writes into at most 19-byte chunks and wait for each characteristic
   write completion before sending the next chunk;
7. preserve packet bytes exactly across BLE chunks and fail the stream on
   disconnect or GATT errors.

The service UUID suffix is not fixed in the vendor example, therefore callers
must discover by prefix and characteristics rather than invent a full UUID.

The Datecs `GET MAX MTU` command is an application-protocol MTU for External
Internet `RECEIVE DATA`; it is separate from the BLE ATT MTU.

## Bulgaria profile

Transaction type `0x10 REFUND` is Romania-only in Pinpad Commands v3 and returns
`DatecsError::NoSupport` in this Bulgaria-targeted module. Purchase with
reference uses transaction type `0x03`, as specified for Bulgaria.

Run host-side protocol tests with:

```sh
sh run-tests.sh
```
