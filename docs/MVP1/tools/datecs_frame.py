import json
def nib(v,n=4): return bytes(0x30+((v>>(4*(n-1-i)))&0xF) for i in range(n))
def frame(seq,cmd,data):
    body=nib(len(data)+0x2A)[:0]  # placeholder
    L=nib(len(data)+0x2A)  # LEN counts: LEN(4)+SEQ+CMD(4)+DATA+05 + 0x20 offset
    core=L+bytes([seq])+nib(cmd)+data+b'\x05'
    return b'\x01'+core+nib(sum(core)&0xFFFF)+b'\x03'
# verify with PM example cmd 43
ex=bytes.fromhex("01 30 30 36 3F 2F 30 30 32 3B 32 31 09 32 31 09 39 38 37 36 09 30 09 34 32 38 09 32 34 2D 30 34 2D 31 39 20 30 38 3A 33 36 3A 32 37 09 30 32 36 33 36 35 37 31 09 09 09 09 44 54 36 33 36 34 39 37 2D 30 30 32 31 2D 30 30 31 30 30 30 31 09 05 30 3D 3E 3F 03".replace(' ',''))
data=ex[1+4+1+4:-1-4-1]
print(len(data),frame(0x2F,43,data)==ex, frame(0x2F,43,data).hex()==ex.hex())
