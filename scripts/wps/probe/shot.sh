#!/bin/bash
# shot.sh DISPLAYNUM OUT.png
xwd -root -display :$1 -silent | python3 -c '
import sys,struct,zlib
d=sys.stdin.buffer.read()
hs=struct.unpack(">I",d[0:4])[0]; w=struct.unpack(">I",d[16:20])[0]; h=struct.unpack(">I",d[20:24])[0]
bpl=struct.unpack(">I",d[48:52])[0]; ncol=struct.unpack(">I",d[76:80])[0]
px=d[hs+ncol*12:]
rows=b"".join(b"\0"+b"".join(bytes((px[y*bpl+x*4+2],px[y*bpl+x*4+1],px[y*bpl+x*4])) for x in range(w)) for y in range(h))
c=lambda t,b: struct.pack(">I",len(b))+t+b+struct.pack(">I",zlib.crc32(t+b)&0xffffffff)
open(sys.argv[1],"wb").write(b"\x89PNG\r\n\x1a\n"+c(b"IHDR",struct.pack(">IIBBBBB",w,h,8,2,0,0,0))+c(b"IDAT",zlib.compress(rows))+c(b"IEND",b""))
' "$2"
